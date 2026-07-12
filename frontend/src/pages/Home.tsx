import { useAuth } from "../lib/auth";
import { Dashboard } from "./Dashboard";
import { TenantHome } from "./TenantHome";

// Home routes "/" to the customer console home for account-scoped users and the
// operator dashboard for platform staff.
export function Home() {
  const { user } = useAuth();
  return user?.accountId ? <TenantHome /> : <Dashboard />;
}
