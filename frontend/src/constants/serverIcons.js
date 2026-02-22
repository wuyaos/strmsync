import { DEFAULT_SERVER_ICON, SERVER_ICON_MAP } from '@/constants/serverIconMap'

const localSvgModules = import.meta.glob('/src/assets/icons/*.svg', {
  eager: true,
  import: 'default'
})
const localPngModules = import.meta.glob('/src/assets/icons/*.png', {
  eager: true,
  import: 'default'
})

export const getServerIcon = (type) => {
  if (!type) return DEFAULT_SERVER_ICON
  return SERVER_ICON_MAP[type] || DEFAULT_SERVER_ICON
}

export const getLocalIconUrl = (name) => {
  if (!name) return ''
  const svgKey = '/src/assets/icons/' + name + '.svg'
  const pngKey = '/src/assets/icons/' + name + '.png'
  return localSvgModules[svgKey] || localPngModules[pngKey] || ''
}

export const getServerIconUrl = (server) => {
  const typeName = server?.type ? String(server.type).toLowerCase() : ''
  const localIcon = getLocalIconUrl(typeName)
  return localIcon || server?.icon || server?.icon_url || server?.ico || server?.favicon || ''
}

export { SERVER_ICON_MAP, localSvgModules, localPngModules }
