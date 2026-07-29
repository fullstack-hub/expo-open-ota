export interface ExpoCredentials {
  token?: string;
}

// The server authenticates publish calls with a publish token only
// (publishTokens entries in EXPO_APPS_JSON). EXPO_TOKEN and the ~/.expo
// eas-login session were api.expo.dev credentials — the server no longer
// understands them, so the CLI no longer sends them.
export function retrieveExpoCredentials(): ExpoCredentials {
  return { token: process.env.OTA_PUBLISH_TOKEN };
}

export function getAuthExpoHeaders(credentials: ExpoCredentials): Record<string, string> {
  if (credentials.token) {
    return {
      Authorization: `Bearer ${credentials.token}`,
    };
  }
  return {};
}
