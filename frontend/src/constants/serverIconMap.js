import Cloudy from '~icons/ep/cloudy'
import Monitor from '~icons/ep/monitor'
import VideoPlay from '~icons/ep/video-play'

export const SERVER_ICON_MAP = {
  local: Monitor,
  clouddrive2: Cloudy,
  openlist: Cloudy,
  emby: VideoPlay,
  jellyfin: VideoPlay,
  plex: VideoPlay
}

export const DEFAULT_SERVER_ICON = Monitor
