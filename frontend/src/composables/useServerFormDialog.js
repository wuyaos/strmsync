import { computed, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { createServer, listDirectories, testServer, testServerTemp, updateServer } from '@/api/servers'
import { usePathDialog, normalizePath } from '@/composables/usePathDialog'

export const useServerFormDialog = (props, emit) => {
  const visible = computed({
    get: () => props.modelValue,
    set: (val) => emit('update:modelValue', val)
  })

  const isEdit = computed(() => !!props.editingServer)
  const isDataMode = computed(() => props.mode === 'data')

  const formRef = ref(null)
  const formData = reactive({
    id: null,
    name: '',
    type: '',
    host: '',
    port: 80,
    api_key: '',
    options: {},
    enabled: true,
    download_rate_per_sec: 0,
    api_rate: 0,
    api_concurrency: 0,
    api_retry_max: 0,
    api_retry_interval_sec: 0
  })

  const dynamicModel = reactive({})
  const autoNameActive = ref(true)
  const testStatus = ref('idle')
  const saving = ref(false)
  const testing = ref(false)

  const formModel = computed(() => {
    if (props.mode === 'data') {
      return { ...formData, ...dynamicModel }
    }
    return formData
  })

  const dataServerTypeOptions = computed(() =>
    props.dataTypeDefs.map((def) => ({
      label: def.label,
      value: def.type,
      description: def.description || ''
    }))
  )

  const serverTypeOptions = computed(() =>
    props.mode === 'data' ? dataServerTypeOptions.value : props.mediaTypeOptions
  )

  const dialogTitle = computed(() => (isEdit.value ? '编辑服务器' : '新增服务器'))

  const buildRemoteRootField = () => ({
    name: 'remote_root',
    type: 'path',
    label: '远程根目录',
    placeholder: '/',
    help: '远程 API 的根路径（用于获取文件列表/信息）',
    required: false,
    default: '/'
  })

  const ensureRemoteRootField = (typeDef) => {
    if (!typeDef) return typeDef
    const type = String(typeDef.type || '').toLowerCase()
    if (type !== 'clouddrive2' && type !== 'openlist') return typeDef
    const sections = Array.isArray(typeDef.sections) ? typeDef.sections : []
    let hasField = false
    for (const section of sections) {
      for (const field of section.fields || []) {
        if (field?.name === 'remote_root') {
          hasField = true
          break
        }
      }
      if (hasField) break
    }
    if (hasField) return typeDef

    const nextSections = sections.map((section) => {
      if (section.id !== 'paths') return section
      return {
        ...section,
        fields: [...(section.fields || []), buildRemoteRootField()]
      }
    })
    const hasPaths = sections.some((section) => section.id === 'paths')
    if (!hasPaths) {
      nextSections.push({
        id: 'paths',
        label: '路径配置',
        fields: [buildRemoteRootField()]
      })
    }

    return {
      ...typeDef,
      sections: nextSections,
      storage: {
        ...(typeDef.storage || {}),
        remote_root: 'options'
      }
    }
  }

  const activeTypeDef = computed(() => {
    if (props.mode !== 'data' || !formData.type) return null
    const def = props.dataTypeDefs.find((item) => item.type === formData.type) || null
    return ensureRemoteRootField(def)
  })

  const currentTypeConfig = computed(() => {
    if (props.mode !== 'media') return null
    if (!formData.type) return null
    return serverTypeOptions.value.find((opt) => opt.value === formData.type)
  })

  const serverTypeHint = computed(() => {
    if (props.mode === 'data') {
      return activeTypeDef.value?.description || ''
    }
    return currentTypeConfig.value?.hint || ''
  })

  const hostPlaceholder = computed(() => '127.0.0.1 或 example.com')
  const needsApiKey = computed(() => currentTypeConfig.value?.needsApiKey || false)
  const apiKeyLabel = computed(() => currentTypeConfig.value?.apiKeyLabel || 'API Key')

  const canTest = computed(() => {
    if (!formData.type) return false
    return true
  })

  const showTestButton = computed(() => !!formData.type && formData.type !== 'local')

  const showRate = computed(() =>
    props.mode === 'data' && ['clouddrive2', 'openlist'].includes(formData.type)
  )

  const testStatusText = computed(() => {
    switch (testStatus.value) {
      case 'running':
        return '测试中'
      case 'success':
        return '连接成功'
      case 'error':
        return '连接失败'
      default:
        return ''
    }
  })

  const baseFormRules = {
    name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
    type: [{ required: true, message: '请选择类型', trigger: 'change' }]
  }

  const mediaFormRules = {
    host: [{ required: true, message: '请输入主机', trigger: 'blur' }],
    port: [
      { required: true, message: '请输入端口', trigger: 'change' },
      {
        validator: (_, value, callback) => {
          if (value < 1 || value > 65535) {
            callback(new Error('端口范围为 1-65535'))
            return
          }
          callback()
        },
        trigger: 'change'
      }
    ],
    options: [
      {
        validator: (_, value, callback) => {
          if (!value) {
            callback()
            return
          }
          try {
            JSON.parse(value)
            callback()
          } catch (error) {
            callback(new Error('Options 必须是合法 JSON'))
          }
        },
        trigger: 'blur'
      }
    ]
  }

  const formRules = computed(() => {
    if (props.mode !== 'data') {
      const rules = { ...baseFormRules, ...mediaFormRules }
      if (needsApiKey.value) {
        rules.api_key = [{ required: true, message: `请输入${apiKeyLabel.value}`, trigger: 'blur' }]
      }
      return rules
    }
    const dynamic = buildDynamicRules(activeTypeDef.value)
    return { ...baseFormRules, ...dynamic }
  })

  const resetDynamicModel = () => {
    for (const key of Object.keys(dynamicModel)) {
      delete dynamicModel[key]
    }
  }

  const resetForm = () => {
    formData.id = null
    formData.name = ''
    formData.type = ''
    formData.host = ''
    formData.port = 80
    formData.api_key = ''
    formData.options = '{}'
    formData.enabled = true
    formData.download_rate_per_sec = 0
    formData.api_rate = 0
    formData.api_concurrency = 0
    formData.api_retry_max = 0
    formData.api_retry_interval_sec = 0
    autoNameActive.value = true
    testStatus.value = 'idle'
    resetDynamicModel()
  }

  const getTypeLabel = () => {
    if (props.mode === 'data') {
      return activeTypeDef.value?.label || formData.type
    }
    return currentTypeConfig.value?.label || formData.type
  }

  const generateDefaultName = () => {
    const raw = getTypeLabel()
    if (!raw) return ''
    const base = raw.replace(/\s+/g, '')
    const pattern = new RegExp(`^${base}(\\d+)$`, 'i')
    let max = 0

    for (const server of props.serverList) {
      if (server.type !== formData.type) continue
      const name = String(server.name || '').replace(/\s+/g, '')
      const match = name.match(pattern)
      if (match) {
        const num = Number(match[1])
        if (!Number.isNaN(num)) max = Math.max(max, num)
      }
    }

    return `${base}${max + 1}`
  }

  const applyDefaultName = () => {
    if (isEdit.value || !formData.type) return
    if (formData.name && !autoNameActive.value) return

    const nextName = generateDefaultName()
    if (nextName) {
      formData.name = nextName
      autoNameActive.value = true
    }
  }

  const handleNameInput = () => {
    if (!isEdit.value) autoNameActive.value = false
  }

  const flattenFields = (typeDef) => {
    return (typeDef?.sections || []).flatMap((section) => section?.fields || [])
  }

  const applyTypeDefaults = (typeDef) => {
    resetDynamicModel()
    if (!typeDef) return
    for (const field of flattenFields(typeDef)) {
      if (field.default !== undefined) {
        dynamicModel[field.name] = field.default
        continue
      }
      if (field.type === 'number') {
        dynamicModel[field.name] = field.min ?? 0
        continue
      }
      dynamicModel[field.name] = ''
    }
  }

  const normalizeOptions = (raw) => {
    if (!raw) return {}
    if (typeof raw === 'object') return raw
    try {
      return JSON.parse(raw)
    } catch (error) {
      return {}
    }
  }

  const hydrateDynamicModel = (row) => {
    const typeDef = activeTypeDef.value
    if (!typeDef) return
    applyTypeDefaults(typeDef)
    const options = normalizeOptions(row.options)
    const storage = typeDef.storage || {}
    for (const field of flattenFields(typeDef)) {
      const target = storage[field.name] || 'options'
      if (target === 'root' && row[field.name] !== undefined) {
        dynamicModel[field.name] = row[field.name]
        continue
      }
      if (target === 'api_key' && row.api_key !== undefined) {
        dynamicModel[field.name] = row.api_key
        continue
      }
      if (target === 'options' && options[field.name] !== undefined) {
        dynamicModel[field.name] = options[field.name]
      }
    }
  }

  const isFieldVisible = (field) => {
    if (!field) return false
    if (field.type === 'hidden') return false
    if (!field.visible_if) return true
    return Object.entries(field.visible_if).every(
      ([key, expected]) => String(dynamicModel[key]) === String(expected)
    )
  }

  const getVisibleFields = (fields) => (fields || []).filter((field) => isFieldVisible(field))
  const isTextField = (field) => field.type === 'text'
  const isPathField = (field) => field?.type === 'path'
  const buildPayload = () => {
    const payload = {
      name: formData.name,
      type: formData.type,
      enabled: formData.enabled,
      download_rate_per_sec: formData.download_rate_per_sec,
      api_rate: formData.api_rate,
      api_concurrency: formData.api_concurrency,
      api_retry_max: formData.api_retry_max,
      api_retry_interval_sec: formData.api_retry_interval_sec
    }

    const typeDef = activeTypeDef.value
    if (!typeDef) {
      return payload
    }

    const options = {}
    const storage = typeDef.storage || {}
    for (const field of flattenFields(typeDef)) {
      if (!isFieldVisible(field)) {
        continue
      }

      const value = dynamicModel[field.name]
      const target = storage[field.name] || 'options'
      if (value === undefined || value === '') {
        continue
      }
      if (target === 'root') {
        payload[field.name] = value
        continue
      }
      if (target === 'api_key') {
        payload.api_key = value
        continue
      }
      options[field.name] = value
    }

    payload.options = options
    return payload
  }

  const buildMediaPayload = () => ({
    name: formData.name,
    type: formData.type,
    host: formData.host,
    port: formData.port,
    api_key: formData.api_key,
    options: normalizeOptions(formData.options),
    enabled: formData.enabled,
    download_rate_per_sec: formData.download_rate_per_sec,
    api_rate: formData.api_rate,
    api_concurrency: formData.api_concurrency,
    api_retry_max: formData.api_retry_max,
    api_retry_interval_sec: formData.api_retry_interval_sec
  })

  const buildDynamicRules = (typeDef) => {
    if (!typeDef) return {}
    const rules = {}
    for (const field of flattenFields(typeDef)) {
      if (!field.required) continue
      rules[field.name] = [
        {
          validator: (_, value, callback) => {
            if (!isFieldVisible(field)) {
              callback()
              return
            }
            if (value === undefined || value === null || value === '') {
              callback(new Error(`请输入${field.label || field.name}`))
              return
            }
            callback()
          },
          trigger: field.type === 'select' ? 'change' : 'blur'
        }
      ]
    }
    return rules
  }

  const handleTypeChange = () => {
    if (props.mode === 'data') {
      applyTypeDefaults(activeTypeDef.value)
      if (!isEdit.value) {
        applyDefaultName()
      }
    } else {
      const config = currentTypeConfig.value
      if (config && config.defaultPort > 0) {
        formData.port = config.defaultPort
      }
      if (!isEdit.value) {
        applyDefaultName()
      }
    }
  }

  const prepareCreate = () => {
    resetForm()
    if (props.mode === 'data' && props.dataTypeDefs.length > 0) {
      formData.type = props.dataTypeDefs[0].type
      applyTypeDefaults(activeTypeDef.value)
      applyDefaultName()
    } else if (props.mode === 'media' && props.mediaTypeOptions.length > 0) {
      const defaultType = props.mediaTypeOptions.find(opt => opt.value === 'emby') || props.mediaTypeOptions[0]
      if (defaultType) {
        formData.type = defaultType.value
        if (defaultType.defaultPort > 0) {
          formData.port = defaultType.defaultPort
        }
      }
      applyDefaultName()
    }
  }

  const prepareEdit = (row) => {
    if (!row) return
    resetForm()
    testStatus.value = 'idle'
    formData.id = row.id
    formData.name = row.name
    formData.type = row.type
    formData.host = row.host
    formData.port = row.port
    formData.api_key = row.api_key || ''
    formData.options = normalizeOptions(row.options)
    formData.enabled = row.enabled !== false
    formData.download_rate_per_sec = row.download_rate_per_sec ?? 0
    formData.api_rate = row.api_rate ?? 0
    formData.api_concurrency = row.api_concurrency ?? 0
    formData.api_retry_max = row.api_retry_max ?? 0
    formData.api_retry_interval_sec = row.api_retry_interval_sec ?? 0
    if (props.mode === 'data' && props.dataTypeDefs.length > 0) {
      hydrateDynamicModel(row)
    }
  }

  const collectTestValidationFields = () => {
    const fields = new Set()
    if (formData.type) fields.add('type')

    if (props.mode === 'media') {
      fields.add('host')
      fields.add('port')
      if (needsApiKey.value) fields.add('api_key')
      return Array.from(fields)
    }

    const typeDef = activeTypeDef.value
    if (!typeDef) return Array.from(fields)
    for (const field of flattenFields(typeDef)) {
      if (!field.required) continue
      if (!isFieldVisible(field)) continue
      fields.add(field.name)
    }
    return Array.from(fields)
  }

  const validateBeforeTest = async () => {
    if (!formRef.value) return true
    const fields = collectTestValidationFields()
    if (fields.length === 0) return true
    try {
      await formRef.value.validateField(fields)
      return true
    } catch (error) {
      return false
    }
  }

  const handleTestConnection = async () => {
    const isValid = await validateBeforeTest()
    if (!isValid) {
      testStatus.value = 'idle'
      return
    }
    testing.value = true
    testStatus.value = 'running'
    try {
      if (formData.id) {
        await testServer(formData.id, formData.type)
      } else {
        const payload = props.mode === 'data'
          ? buildPayload()
          : buildMediaPayload()
        await testServerTemp(payload)
      }
      ElMessage.success('连接测试成功！')
      testStatus.value = 'success'
    } catch (error) {
      testStatus.value = 'error'
    } finally {
      testing.value = false
    }
  }

  const handleSave = async () => {
    try {
      await formRef.value?.validate()
      saving.value = true
      const payload = props.mode === 'data' ? buildPayload() : buildMediaPayload()
      if (isEdit.value) {
        await updateServer(formData.id, payload)
        ElMessage.success('服务器已更新')
      } else {
        await createServer(payload)
        ElMessage.success('服务器已创建')
      }
      visible.value = false
      emit('saved')
    } catch (error) {
      if (error?.message) {
        console.error('保存服务器失败:', error)
      }
    } finally {
      saving.value = false
    }
  }

  const pathDlg = usePathDialog({
    loader: (payload) => listDirectories({
      path: payload?.path,
      limit: payload?.limit,
      offset: payload?.offset,
      mode: 'local'
    }),
    onError: () => ElMessage.error('加载目录失败')
  })
  const pathDialogField = ref(null)

  const openPathDialog = async (field) => {
    if (!field) return
    pathDialogField.value = field
    const currentValue = dynamicModel[field.name] || '/'
    await pathDlg.open({ mode: 'single', root: '/', path: currentValue })
    const normalized = normalizePath(currentValue)
    if (normalized !== '/') {
      pathDlg.selectedName.value = normalized
    } else {
      pathDlg.clearSelection()
    }
  }

  const handlePathSelect = (name) => {
    pathDlg.selectRow(name)
  }

  const handlePathConfirm = () => {
    if (!pathDialogField.value) return
    const selectedPath = pathDlg.getSelectedSingle()
    dynamicModel[pathDialogField.value.name] = selectedPath
    pathDlg.close()
  }

  watch(() => props.modelValue, (val) => {
    if (!val) return
    if (props.editingServer) {
      prepareEdit(props.editingServer)
    } else {
      prepareCreate()
    }
  })

  watch(() => props.dataTypeDefs, (defs) => {
    if (!visible.value || !props.editingServer || props.mode !== 'data') return
    if (defs.length > 0) {
      hydrateDynamicModel(props.editingServer)
    }
  })

  return {
    visible,
    isEdit,
    isDataMode,
    formRef,
    formData,
    dynamicModel,
    formModel,
    formRules,
    serverTypeOptions,
    dialogTitle,
    activeTypeDef,
    serverTypeHint,
    hostPlaceholder,
    needsApiKey,
    apiKeyLabel,
    showTestButton,
    testStatus,
    testStatusText,
    testing,
    canTest,
    showRate,
    saving,
    openPathDialog,
    handlePathSelect,
    handlePathConfirm,
    handleTestConnection,
    handleSave,
    handleTypeChange,
    handleNameInput,
    getVisibleFields,
    isPathField,
    isTextField,
    pathDlg
  }
}
