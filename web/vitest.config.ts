import { defineVitestConfig } from '@nuxt/test-utils/config'

// Default environment stays node (utils/queries tests). Component tests opt
// into the Nuxt environment per file via the *.nuxt.test.ts suffix.
export default defineVitestConfig({
  test: {
    environment: 'node',
    include: ['tests/**/*.test.ts'],
    environmentOptions: {
      nuxt: {
        domEnvironment: 'happy-dom',
      },
    },
  },
})
