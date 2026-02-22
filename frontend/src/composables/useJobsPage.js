import { onMounted, onUnmounted, ref } from "vue"
import { useJobForm } from "@/composables/useJobForm"
import { useJobList } from "@/composables/useJobList"
import { usePathBrowser } from "@/composables/usePathBrowser"

export const useJobsPage = () => {
  const isActive = ref(true)

  const jobList = useJobList({ isActive })
  const jobForm = useJobForm({
    isActive,
    onSaved: () => jobList.loadJobs()
  })
  const pathBrowser = usePathBrowser({
    formData: jobForm.formData,
    currentServer: jobForm.currentServer,
    isEdit: jobForm.isEdit
  })

  jobForm.setAfterReset(pathBrowser.resetAutoMediaDir)
  jobForm.setAfterServerChange(pathBrowser.applyDefaultMediaDir)

  onMounted(() => {
    jobForm.loadServers()
    jobList.loadJobs()
  })

  onUnmounted(() => {
    isActive.value = false
  })

  return {
    ...jobList,
    ...jobForm,
    ...pathBrowser
  }
}
