# RISK: 퍼블리시 엔드포인트 인증 — api.expo.dev 제거 시 publish 토큰이 필요한 이유

> 작성일: 2026-06-11 (eas-delete 구조로 갱신) / 관련 구현: `internal/services/expo_local.go`

## 요약 (TL;DR)

api.expo.dev 연동을 제거하면 **퍼블리시 엔드포인트의 신원 검증 수단이 통째로 사라진다.** 대체 없이 제거할 경우 서버 URL과 appId만 아는 누구나 유저 단말에 임의 코드를 푸시할 수 있는 공급망 공격(supply-chain attack)에 노출된다. publish 토큰은 새 기능이 아니라 **api.expo.dev가 해주던 "퍼블리셔 신원 확인"의 대체물**이며, 영향 범위는 쓰기(퍼블리시) 경로 5개뿐이다. 토큰은 **서버가 형식을 전혀 모르는 불투명 시크릿**으로, CI Secret(`OTA_PUBLISH_TOKEN`)과 서버 config(`EXPO_APPS_JSON`의 `publishTokens`)에 같은 값을 둔다.

---

## 1. 위협 모델: 왜 무인증이면 안 되는가

이 서버의 코드사이닝 구조 때문에 퍼블리시 경로의 무인증은 치명적이다:

1. 코드사이닝 **개인키는 서버에 있다** (`internal/keyStore`, 앱별 local/AWS-SM/env 모드)
2. 서버는 업로드된 번들을 검증 없이 **그대로 서명해 배포**한다 (`expo-signature` 헤더, manifest_handler.go)
3. 클라이언트(expo-updates)는 그 서명을 신뢰하고 번들을 실행한다

따라서 퍼블리시 엔드포인트가 뚫리면 공격자의 번들이 **정상 서명을 달고** 전체 유저 단말에 OTA로 배포된다. 앱 스토어 심사도, 별도 게이트도 없다.

## 2. 업스트림(EAS 연동)의 인증 구조 — 제거된 것

업스트림 expo-open-ota의 쓰기 엔드포인트는 `ValidateExpoAuth`로 보호됐다: CLI가 `eas login` 자격증명을 보내면 서버가 api.expo.dev `me` 쿼리 2회로 "요청자 == 앱 소유 Expo 계정"을 대조하는 **신원 기반 인증**이다. 이 빌드는 expo.go를 삭제했으므로 그 수단이 사라졌고, 아래의 토큰 인증이 대체한다.

## 3. 대체: publish 토큰 (불투명 시크릿 비교)

```
[발급 시 1회]
openssl rand -hex 32 (권고 — 서버는 생성 방식을 모름)
 ├─ CI Secret (OTA_PUBLISH_TOKEN)
 └─ EXPO_APPS_JSON의 publishTokens          ← 같은 값을 양쪽에 등록

[매 배포 시]
CI → OTA_PUBLISH_TOKEN 주입 → eoas publish → Authorization: Bearer <토큰>
   → 서버: 저장 항목과 constant-time 비교(crypto/subtle) → 통과 시 업로드 허용
```

- `publishTokens: ["<token>"]` 배열 (복수 허용 — 무중단 회전용)
- 항목은 **완전한 불투명 문자열** — 서버는 해시·인코딩·생성 방식(랜덤, 공통 관리 암호, 미래의 다른 유래)을 전혀 모르고 수신 값을 그대로 대조만 한다. 형식 검증 없음. 인증 강도는 오로지 토큰의 엔트로피
- 배열이 비면 해당 앱은 publish 불가(읽기 전용)
- 검증이 appId 단위이므로 cross-tenant(다른 앱에 대한 퍼블리시) 차단은 기존과 동등

서버에 토큰이 그대로 저장되지만 별도 위협이 되지 않는 이유: **EXPO_APPS_JSON은 이미 코드사이닝 키 설정을 담는 Secret**이다. 이걸 읽을 수 있는 공격자는 publish 토큰 없이도 서명키로 직접 악성 업데이트를 만들 수 있는 위치이므로, 이 Secret의 열람 통제가 무너지면 토큰 저장 형식과 무관하게 끝이다. 따라서 해시 저장(유출 피해 축소)은 실효가 없어 채택하지 않았다 — §5 참고.

### 인증 모델의 변화 (트레이드오프)

| | 업스트림 (EAS) | 이 빌드 |
|---|---|---|
| 인증 종류 | **신원 기반** (Expo 계정 소유 증명) | **소지 기반** (토큰 보유 = 권한) |
| 검증 주체 | api.expo.dev (`me` 쿼리 2회) | 서버 자체 (불투명 문자열 대조) |
| CI에 필요한 Secret | `EXPO_TOKEN` | `OTA_PUBLISH_TOKEN` (자체 발급) |
| 서버 측 보관물 | `accessToken` 원문 | 토큰 (서명키 설정과 같은 Secret에 동거) |
| 외부 종속 | Expo 장애 시 publish/manifest 불가 | 없음 |
| 토큰 유출 시 | Expo 대시보드에서 즉시 폐기 | 항목 교체 + 재배포 (회전 절차 필수) |

소지 기반으로 약화되는 만큼 **토큰 보관처(CI Secret + EKS Secret) 관리가 보안의 전부**가 된다: git·로그·위키 금지, 정기 회전(새 항목 추가 → CI 교체 → 구 항목 제거).

## 4. 영향 범위: 쓰기(퍼블리시) 경로에만 해당

| 경로 | 인증 |
|------|------|
| `POST /{appId}/requestUploadUrl/{branch}` | publish 토큰 ✅ |
| `PUT /{appId}/uploadLocalFile` | publish 토큰 (+ 업로드 JWT) ✅ |
| `POST /{appId}/markUpdateAsUploaded/{branch}` | publish 토큰 ✅ |
| `POST /{appId}/rollback/{branch}` | publish 토큰 ✅ |
| `POST /{appId}/republish/{branch}` | publish 토큰 ✅ |
| `GET /manifest`, `GET /assets` | **무인증** (단말 공개 경로 — 기존과 동일) |
| `/api/*` (대시보드) | `ADMIN_PASSWORD` + JWT (별도 체계) |

각주: 대시보드 API의 `Use-Expo-Auth: true` 우회 경로는 publish 토큰으로 동작한다. 실제 대시보드 프론트엔드는 admin JWT를 쓰므로 운영 영향 없음.

## 5. 검토했던 대안과 기각 사유

| 대안 | 평가 |
|------|------|
| **무인증** | 불가 — §1의 공급망 공격에 그대로 노출 |
| **대시보드 admin JWT 재사용** | 전 앱 공용 크리덴셜이라 앱별 격리 상실. 2시간 만료 구조라 CI 비대화형에 부적합 |
| **네트워크 격리 단독** | 보완책으로는 유효하나 같은 서버가 `/manifest`를 공개 서빙하므로 단독으로는 부족. 설정 드리프트 시 조용히 무인증이 됨 |
| **서버에 해시만 저장 (SHA-256 등)** | 초기 구현에 있었으나 제거. 지키는 자산(publish 권한)보다 큰 자산(서명키 설정)이 같은 Secret에 있어 유출 시나리오에서 실효가 없고, 서버가 알고리즘을 알아야 해 토큰 불투명성이 깨짐. 유출 방어가 진짜 목표라면 정답은 비대칭(아래)이지 해시가 아님 |
| **비대칭 키 (CI 서명 JWT)** | 서버에 비밀이 안 남는 유일한 구조라 원리상 가장 강함. 그러나 주 유출 지점인 CI가 뚫리면(개인키 보관처) 동일하게 끝이고, 요청 서명·리플레이 방지 등 CLI/서버 복잡도가 과함. 외부 위임 요구 생기면 그때 추가 |
| **mTLS / OIDC** | CI·CLI 복잡도가 토큰 대비 과함. 필요 시 토큰 위에 추가 |

## 6. 운영 체크리스트

- [ ] 토큰 발급 (앱별 1회 + 회전 시) — 고엔트로피 랜덤 권고:
  ```bash
  TOKEN=$(openssl rand -hex 32)
  ```
- [ ] 같은 값을 CI Secret(`OTA_PUBLISH_TOKEN`)과 EXPO_APPS_JSON의 `publishTokens` 양쪽에 등록 — git·로그·위키 금지
- [ ] 회전: 새 항목 추가 → CI 교체 → 구 항목 제거
- [ ] 401 급증 모니터링 (회전 누락 또는 공격 시도 신호)
- [ ] EAS 연동 복귀가 필요하면: expo_local.go 도입 PR을 git revert 후 재배포
