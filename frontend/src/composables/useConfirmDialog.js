import { createApp, defineComponent, h, reactive } from 'vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'

const state = reactive({
  visible: false,
  title: '确认操作',
  message: '',
  type: 'info',
  items: [],
  confirmText: '确认',
  cancelText: '取消',
  resolve: null
})

let app = null
let container = null

const ensureApp = () => {
  if (app) return
  container = document.createElement('div')
  document.body.appendChild(container)
  app = createApp(defineComponent({
    setup() {
      return () =>
        h(ConfirmDialog, {
          modelValue: state.visible,
          'onUpdate:modelValue': (val) => { state.visible = val },
          title: state.title,
          message: state.message,
          type: state.type,
          items: state.items,
          confirmText: state.confirmText,
          cancelText: state.cancelText,
          onConfirm: () => {
            state.visible = false
            state.resolve?.(true)
            state.resolve = null
          },
          onCancel: () => {
            state.visible = false
            state.resolve?.(false)
            state.resolve = null
          }
        })
    }
  }))
  app.mount(container)
}

export const confirmDialog = (options = {}) => {
  ensureApp()
  if (state.visible && state.resolve) {
    state.resolve(false)
  }
  state.title = options.title || '确认操作'
  state.message = options.message || ''
  state.type = options.type || 'info'
  state.items = Array.isArray(options.items) ? options.items : []
  state.confirmText = options.confirmText || '确认'
  state.cancelText = options.cancelText || '取消'

  return new Promise((resolve) => {
    state.resolve = resolve
    state.visible = true
  })
}
