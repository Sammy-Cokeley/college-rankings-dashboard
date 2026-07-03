export type Theme = 'dark' | 'light'

// Cookie-backed theme choice. No cookie = follow the OS preference (handled in
// CSS via prefers-color-scheme; dark is the base). A cookie makes the choice
// explicit and SSR renders the right theme with no flash: the data-theme
// attribute is emitted into the server-rendered <html>.
export function useTheme() {
  const theme = useCookie<Theme | null>('theme', {
    default: () => null,
    maxAge: 60 * 60 * 24 * 365,
    sameSite: 'lax',
  })

  useHead({
    htmlAttrs: {
      'data-theme': computed(() => theme.value ?? undefined),
    },
  })

  function toggle() {
    const current =
      theme.value ??
      (window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark')
    theme.value = current === 'dark' ? 'light' : 'dark'
  }

  return { theme, toggle }
}
