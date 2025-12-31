import NextAuth, { NextAuthOptions } from "next-auth";
import KeycloakProvider from "next-auth/providers/keycloak";

// Docker/Internal URL (For Server-to-Server communication)
const KEYCLOAK_INTERNAL_URL =
  process.env.KEYCLOAK_INTERNAL_URL || "http://keycloak:8080";
// Public/Browser URL (For User Redirection and Token Issuer Match)
const KEYCLOAK_PUBLIC_URL =
  process.env.KEYCLOAK_PUBLIC_URL || "http://localhost:8080";
const REALM = "rice-search";

export const authOptions: NextAuthOptions = {
  providers: [
    KeycloakProvider({
      clientId: process.env.KEYCLOAK_CLIENT_ID || "rice-search",
      clientSecret: process.env.KEYCLOAK_CLIENT_SECRET || "secret",

      // 1. Validation: Expect token 'iss' to be localhost (Browser URL)
      issuer: `${KEYCLOAK_PUBLIC_URL}/realms/${REALM}`,

      // 2. Discovery: Fetch metadata from Internal Docker Network
      wellKnown: `${KEYCLOAK_INTERNAL_URL}/realms/${REALM}/.well-known/openid-configuration`,

      // 3. Overrides: Ensure flows use correct networks
      authorization: {
        url: `${KEYCLOAK_PUBLIC_URL}/realms/${REALM}/protocol/openid-connect/auth`,
        params: { scope: "openid email profile" },
      },
      token: `${KEYCLOAK_INTERNAL_URL}/realms/${REALM}/protocol/openid-connect/token`,
      userinfo: `${KEYCLOAK_INTERNAL_URL}/realms/${REALM}/protocol/openid-connect/userinfo`,
    }),
  ],
  callbacks: {
    async jwt({ token, account }) {
      if (account) {
        token.accessToken = account.access_token;
      }
      return token;
    },
    async session({ session, token }) {
      session.accessToken = token.accessToken as string;
      return session;
    },
  },
};

const handler = NextAuth(authOptions);

export { handler as GET, handler as POST };
