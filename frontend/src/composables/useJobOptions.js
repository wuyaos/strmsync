export const parseJobOptions = (options) => {
  if (!options) return {}
  if (typeof options === "object") return options
  if (typeof options === "string") {
    try {
      return JSON.parse(options)
    } catch (error) {
      return {}
    }
  }
  return {}
}
