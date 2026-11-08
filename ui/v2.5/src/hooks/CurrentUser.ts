import * as GQL from "src/core/generated-graphql";

const roleLevel: Record<GQL.UserRole, number> = {
  [GQL.UserRole.Viewer]: 1,
  [GQL.UserRole.Editor]: 2,
  [GQL.UserRole.Admin]: 3,
};

export function roleAtLeast(
  role: GQL.UserRole | undefined,
  min: GQL.UserRole
): boolean {
  if (!role) return false;
  return roleLevel[role] >= roleLevel[min];
}

/**
 * Returns the current user and their permissions. While loading, the user is
 * treated as an admin so that the UI does not flicker for single-user setups;
 * the server enforces the actual permissions.
 */
export function useCurrentUser() {
  const { data, loading } = GQL.useMeQuery({ fetchPolicy: "cache-first" });

  const me = data?.me;
  const role = me?.role ?? GQL.UserRole.Admin;

  return {
    me,
    loading,
    // true if user accounts are configured
    multiUser: !!me?.username,
    isAdmin: roleAtLeast(role, GQL.UserRole.Admin),
    isEditor: roleAtLeast(role, GQL.UserRole.Editor),
  };
}
