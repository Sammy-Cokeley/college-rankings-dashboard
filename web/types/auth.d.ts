// Augments nuxt-auth-utils' session types (see its README "Vue Composable" /
// "Server Utils" sections). Only what's needed to recognize a returning
// user — no `secure` data, since password verification happens once at
// login/signup and nothing sensitive needs to ride along in the session.
declare module '#auth-utils' {
  interface User {
    id: number
    email: string
    displayName: string | null
  }

  interface UserSession {
    // epoch the session was issued under; checked against users.session_epoch
    // on every request so bumping the column invalidates old sessions
    // (schema.md §9).
    epoch: number
  }
}

export {}
