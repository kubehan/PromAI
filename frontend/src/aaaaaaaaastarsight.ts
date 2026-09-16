import type { Router } from 'vue-router'
import type { App } from 'vue'

export function useStarSight() {
  const sdk = (window as any).StarSight
  if (!sdk) {
    console.warn('[StarSight] SDK not loaded — did you forget <script src="/starsight-sdk.js">?')
    return null
  }
  return sdk
}

export function reportRouteChange(route: string, title?: string) {
  const sdk = useStarSight()
  if (!sdk) return
  if (title) {
    document.title = title
  }
  sdk.setPerformance({ pagePath: window.location.href })
  const sessionId = sdk.getSessionId()
  if (sessionId) {
    const body = {
      appId: sdk._config?.appId || '',
      pageUrl: window.location.href,
      sessionId,
      events: [{ type: 'pageview', target: route, timestamp: Date.now() }]
    }
    try {
      const url = (sdk._config?.gateway || '').replace(/\/+$/, '') + '/api/v1/collect/session'
      navigator.sendBeacon(url, new Blob([JSON.stringify(body)], { type: 'application/json' }))
    } catch { /* silent */ }
  }
}

export function reportVueError(err: unknown) {
  const sdk = useStarSight()
  if (!sdk) return
  const msg = err instanceof Error ? err.message : String(err)
  const stack = err instanceof Error ? err.stack : ''
  sdk.reportError(msg, stack)
}

export function installStarSightVue(app: App<Element>, router: Router) {
  const sdk = useStarSight()
  if (!sdk) {
    console.warn('[StarSight] Vue integration skipped: SDK not loaded')
    return
  }

  router.afterEach((to) => {
    reportRouteChange(to.path, typeof to.meta?.title === 'string' ? to.meta.title : undefined)
  })

  app.config.errorHandler = (err, _instance, _info) => {
    reportVueError(err)
    console.error('[StarSight] Vue error:', err)
  }

  window.addEventListener('error', (event) => {
    if (event.error && sdk._config?.jsErrors) return
    reportVueError(event.error || event.message)
  })

  console.log('[StarSight] Vue integration installed')
}
