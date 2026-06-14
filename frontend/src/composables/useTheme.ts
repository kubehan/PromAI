import { ref, watch } from 'vue'

export type ThemeName = 'dark' | 'light' | 'cyber' | 'minimal'

interface ThemeOption {
  value: ThemeName
  label: string
  icon: string
  desc: string
}

export const themeOptions: ThemeOption[] = [
  { value: 'dark', label: '深邃暗色', icon: '🌙', desc: '深色背景，护眼低功耗' },
  { value: 'light', label: '明亮浅色', icon: '☀️', desc: '浅色背景，清晰明亮' },
  { value: 'cyber', label: '赛博霓虹', icon: '💠', desc: '霓虹高饱和，赛博朋克风格' },
  { value: 'minimal', label: '极简白', icon: '⬜', desc: '纯白极简，干净利落' },
]

const STORAGE_KEY = 'promai_theme'

function loadTheme(): ThemeName {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved && ['dark', 'light', 'cyber', 'minimal'].includes(saved)) {
      return saved as ThemeName
    }
  } catch {}
  return 'dark'
}

function applyTheme(name: ThemeName) {
  document.documentElement.setAttribute('data-theme', name)
}

const currentTheme = ref<ThemeName>(loadTheme())

watch(currentTheme, (val) => {
  localStorage.setItem(STORAGE_KEY, val)
  applyTheme(val)
}, { immediate: true })

export function useTheme() {
  function setTheme(name: ThemeName) {
    currentTheme.value = name
  }

  return {
    currentTheme,
    setTheme,
    themeOptions,
  }
}
