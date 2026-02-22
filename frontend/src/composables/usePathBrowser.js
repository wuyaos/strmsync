import { computed, ref } from "vue"
import { ElMessage } from "element-plus"
import { listDirectories } from "@/api/servers"
import { usePathDialog, normalizePath, joinPath } from "@/composables/usePathDialog"

export const usePathBrowser = ({ formData, currentServer, isEdit }) => {
  const pathDialogField = ref("")
  const autoMediaDir = ref("")

  const toRelativePath = (path, root) => {
    const normalizedPath = normalizePath(path || "/")
    const normalizedRoot = normalizePath(root || "/")
    if (!normalizedRoot || normalizedRoot === "/") {
      return normalizedPath.replace(/^\/+/, "")
    }
    if (normalizedPath === normalizedRoot) {
      return "."
    }
    if (normalizedPath.startsWith(`${normalizedRoot}/`)) {
      return normalizedPath.slice(normalizedRoot.length + 1)
    }
    return normalizedPath.replace(/^\/+/, "")
  }

  const normalizeExcludeInput = (value) => {
    const trimmed = String(value || "").trim()
    if (!trimmed) return ""
    if (trimmed === ".") return "."
    if (trimmed.startsWith("./")) return trimmed.slice(2)
    if (trimmed.startsWith("/")) {
      return toRelativePath(trimmed, formData.media_dir || "/")
    }
    return trimmed
  }

  const excludeDirsText = computed({
    get: () => formData.exclude_dirs.join(", "),
    set: (value) => {
      if (!value) {
        formData.exclude_dirs = []
        return
      }
      formData.exclude_dirs = value
        .split(",")
        .map(item => normalizeExcludeInput(item))
        .filter(Boolean)
    }
  })

  const isLocalLikePath = (value) => {
    const raw = String(value || "").trim()
    if (!raw) return false
    if (/^[a-zA-Z]:[\\/]/.test(raw)) return true
    return raw.startsWith("/mnt/")
  }

  const resolveAccessRoot = () => {
    const server = currentServer.value
    if (!server) return "/"
    const accessPath = normalizePath(server.accessPath || "")
    if (server.type === "openlist" && !String(server.accessPath || "").trim()) {
      return "/"
    }
    return accessPath
  }

  const applyDefaultMediaDir = () => {
    if (isEdit.value) return
    if (formData.media_dir && formData.media_dir !== autoMediaDir.value) return
    const root = resolveAccessRoot()
    formData.media_dir = root
    autoMediaDir.value = root
  }

  const resetAutoMediaDir = () => {
    autoMediaDir.value = ""
  }

  const buildDirectoryParams = (input) => {
    const path = typeof input === "string" ? input : input?.path
    const limit = typeof input === "object" ? input?.limit : undefined
    const offset = typeof input === "object" ? input?.offset : undefined
    const forceApi = typeof input === "object" ? input?.forceApi : false
    const server = currentServer.value
    if (!server) {
      return { path, mode: "local", limit, offset }
    }
    if (!forceApi && (server.type === "local" || isLocalLikePath(path) || String(server.accessPath || "").trim())) {
      return { path, mode: "local", limit, offset }
    }
    return {
      path,
      limit,
      offset,
      mode: "api",
      type: server.type,
      host: server.host,
      port: server.port,
      apiKey: server.api_key || server.apiKey
    }
  }

  const pathDlg = usePathDialog({
    loader: (payload) => listDirectories(buildDirectoryParams(payload)),
    onError: () => ElMessage.error("加载目录失败")
  })

  const toAbsolutePath = (value, root) => {
    if (!value || value === ".") return normalizePath(root || "/")
    if (String(value).startsWith("/")) return normalizePath(value)
    return normalizePath(joinPath(root || "/", value))
  }

  const resolveDialogRoot = (field) => {
    if (field === "media_dir") {
      return resolveAccessRoot()
    }
    if (field === "remote_root") {
      return "/"
    }
    if (field === "exclude_dirs" && formData.media_dir) {
      return normalizePath(formData.media_dir)
    }
    if (field === "exclude_dirs") {
      return resolveAccessRoot()
    }
    return "/"
  }

  const openPathDialog = async (field, options = {}) => {
    if (field === "media_dir" && !currentServer.value) {
      ElMessage.warning("请先选择数据服务器")
      return
    }
    pathDialogField.value = field
    const dialogRoot = resolveDialogRoot(field)
    const initialPath = field === "exclude_dirs" || field === "media_dir"
      ? dialogRoot
      : (typeof formData[field] === "string" ? formData[field] : "/")
    await pathDlg.open({
      mode: options.multiple ? "multi" : "single",
      root: dialogRoot,
      path: initialPath || dialogRoot,
      extra: { forceApi: options.forceApi }
    })

    if (options.multiple) {
      pathDlg.selectedNames.value = (formData.exclude_dirs || [])
        .map(item => toAbsolutePath(item, dialogRoot))
        .filter(Boolean)
    } else if (typeof formData[field] === "string" && formData[field]) {
      const normalized = normalizePath(formData[field])
      if (normalized !== "/" && normalized !== dialogRoot) {
        pathDlg.selectedName.value = normalized
      }
    }
  }

  const handlePathSelect = (name) => {
    pathDlg.selectRow(name)
  }

  const handlePathToggle = (name) => {
    pathDlg.toggleRow(name)
  }

  const handlePathConfirm = () => {
    if (pathDlg.mode.value === "multi") {
      const root = pathDlg.root.value || formData.media_dir || "/"
      const selected = pathDlg.getSelectedMulti()
      formData.exclude_dirs = selected
        .map(item => toRelativePath(item, root))
        .filter(Boolean)
      pathDlg.close()
      return
    }

    if (!pathDialogField.value) return
    const selectedPath = pathDlg.getSelectedSingle()
    formData[pathDialogField.value] = selectedPath
    pathDlg.close()
  }

  return {
    pathDlg,
    excludeDirsText,
    openPathDialog,
    handlePathSelect,
    handlePathToggle,
    handlePathConfirm,
    applyDefaultMediaDir,
    resetAutoMediaDir
  }
}
