export const MEDIA_SERVER_TYPE_OPTIONS = [
  {
    label: 'Emby',
    value: 'emby',
    description: '媒体服务器',
    defaultPort: 8096,
    needsApiKey: true,
    apiKeyLabel: 'API Key',
    hint: 'Emby Server，需要在设置中生成 API 密钥'
  },
  {
    label: 'Jellyfin',
    value: 'jellyfin',
    description: '开源媒体服务器',
    defaultPort: 8096,
    needsApiKey: true,
    apiKeyLabel: 'API Key',
    hint: 'Jellyfin Server，需要在设置中生成 API 密钥'
  },
  {
    label: 'Plex',
    value: 'plex',
    description: '媒体服务器',
    defaultPort: 32400,
    needsApiKey: true,
    apiKeyLabel: 'X-Plex-Token',
    hint: 'Plex Media Server，需要获取 X-Plex-Token'
  }
]
