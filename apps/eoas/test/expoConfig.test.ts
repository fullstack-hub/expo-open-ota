import { ExpoConfig } from '@expo/config';
import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { getAppIdFromUpdateUrl, requireExpoAppId } from '../src/lib/expoConfig';

describe('getAppIdFromUpdateUrl', () => {
  it('경로에 app id 가 있으면 추출한다', () => {
    assert.equal(
      getAppIdFromUpdateUrl('https://ota.juvisdev.co.kr/aiconsulting/manifest'),
      'aiconsulting'
    );
  });

  it('top-level /manifest 는 app id 없음(undefined)', () => {
    assert.equal(getAppIdFromUpdateUrl('https://ota.juvisdev.co.kr/manifest'), undefined);
  });

  it('트레일링 슬래시가 있어도 추출한다', () => {
    assert.equal(
      getAppIdFromUpdateUrl('https://ota.juvisdev.co.kr/aiconsulting/manifest/'),
      'aiconsulting'
    );
  });

  it('경로 접두사가 있어도 manifest 직전 세그먼트를 쓴다', () => {
    assert.equal(getAppIdFromUpdateUrl('https://host/api/aiconsulting/manifest'), 'aiconsulting');
  });

  it('쿼리/해시는 무시한다', () => {
    assert.equal(
      getAppIdFromUpdateUrl('https://host/aiconsulting/manifest?foo=1#x'),
      'aiconsulting'
    );
  });

  it('updateUrl 이 undefined 면 undefined', () => {
    assert.equal(getAppIdFromUpdateUrl(undefined), undefined);
  });

  it('잘못된 URL 이면 undefined', () => {
    assert.equal(getAppIdFromUpdateUrl('not a url'), undefined);
  });

  it('manifest 로 끝나지 않으면 undefined', () => {
    assert.equal(getAppIdFromUpdateUrl('https://host/aiconsulting/assets'), undefined);
  });
});

describe('requireExpoAppId', () => {
  it('URL 경로의 app id 가 expo-app-id 헤더보다 우선한다', () => {
    const config = {
      updates: {
        url: 'https://ota.juvisdev.co.kr/aiconsulting/manifest',
        // 헤더에는 낡은 projectId UUID 가 남아 있어도 URL 이 이긴다.
        requestHeaders: { 'expo-app-id': '74fb6c1c-7c79-40a9-b5ce-33a1663c61d6' },
      },
    } as unknown as ExpoConfig;
    assert.equal(requireExpoAppId(config), 'aiconsulting');
  });

  it('경로에 app id 가 없으면 expo-app-id 헤더로 fallback 한다', () => {
    const config = {
      updates: {
        url: 'https://ota.juvisdev.co.kr/manifest',
        requestHeaders: { 'expo-app-id': 'myheaderid' },
      },
    } as unknown as ExpoConfig;
    assert.equal(requireExpoAppId(config), 'myheaderid');
  });
});
