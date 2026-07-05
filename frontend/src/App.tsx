import { Center } from "@mantine/core";
import type { ReactNode } from "react";
import { Navigate, Route, Routes, useLocation } from "react-router-dom";
import { Layout } from "./components/Layout";
import { ForbiddenState, LoadingState } from "./components/States";
import { useAuth } from "./lib/auth";
import { Login } from "./pages/Login";
import { Dashboard } from "./pages/Dashboard";
import { LiveDashboard } from "./pages/LiveDashboard";
import { Signals } from "./pages/Signals";
import { SignalDetail } from "./pages/SignalDetail";
import { Entities } from "./pages/Entities";
import { Analytics } from "./pages/Analytics";
import { Sources } from "./pages/Sources";
import { SourceDetail } from "./pages/SourceDetail";
import { Coverage } from "./pages/Coverage";
import { Articles } from "./pages/Articles";
import { ArticleDetail } from "./pages/ArticleDetail";
import { RawItems } from "./pages/RawItems";
import { RawItemDetail } from "./pages/RawItemDetail";
import { Deliveries } from "./pages/Deliveries";
import { DeliveryDetail } from "./pages/DeliveryDetail";
import { Subscriptions } from "./pages/Subscriptions";
import { Subscribers } from "./pages/Subscribers";
import { Taxonomy } from "./pages/Taxonomy";
import { Jobs } from "./pages/Jobs";
import { Users } from "./pages/Users";
import { Teams } from "./pages/Teams";
import { Account } from "./pages/Account";
import { Settings } from "./pages/Settings";
import { Connectors } from "./pages/Connectors";
import { ApiKeys } from "./pages/ApiKeys";
import { AuditLog } from "./pages/AuditLog";

function RequireAuth() {
  const { user, loading } = useAuth();
  const loc = useLocation();
  if (loading) return <Center mih="100vh"><LoadingState /></Center>;
  if (!user) return <Navigate to="/login" state={{ from: loc.pathname }} replace />;
  return <Layout />;
}

// RequirePerm gates a route on a permission. Directly navigating to a route the
// user lacks access to renders an "Access denied" page instead of the component
// (defence-in-depth alongside the nav gating and the server-side authz).
function RequirePerm({ perm, children }: { perm: string; children: ReactNode }) {
  const { hasPerm } = useAuth();
  return hasPerm(perm) ? <>{children}</> : <ForbiddenState />;
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route element={<RequireAuth />}>
        <Route path="/" element={<Dashboard />} />
        <Route path="/live" element={<LiveDashboard />} />
        <Route path="/signals" element={<RequirePerm perm="signals:read"><Signals /></RequirePerm>} />
        <Route path="/signals/:id" element={<RequirePerm perm="signals:read"><SignalDetail /></RequirePerm>} />
        <Route path="/entities" element={<RequirePerm perm="signals:read"><Entities /></RequirePerm>} />
        <Route path="/analytics" element={<RequirePerm perm="analytics:read"><Analytics /></RequirePerm>} />
        <Route path="/sources" element={<RequirePerm perm="sources:read"><Sources /></RequirePerm>} />
        <Route path="/sources/:id" element={<RequirePerm perm="sources:read"><SourceDetail /></RequirePerm>} />
        <Route path="/coverage" element={<RequirePerm perm="sources:read"><Coverage /></RequirePerm>} />
        <Route path="/articles" element={<RequirePerm perm="signals:read"><Articles /></RequirePerm>} />
        <Route path="/articles/:id" element={<RequirePerm perm="signals:read"><ArticleDetail /></RequirePerm>} />
        <Route path="/raw-items" element={<RequirePerm perm="signals:read"><RawItems /></RequirePerm>} />
        <Route path="/raw-items/:id" element={<RequirePerm perm="signals:read"><RawItemDetail /></RequirePerm>} />
        <Route path="/deliveries" element={<RequirePerm perm="deliveries:read"><Deliveries /></RequirePerm>} />
        <Route path="/deliveries/:id" element={<RequirePerm perm="deliveries:read"><DeliveryDetail /></RequirePerm>} />
        <Route path="/subscriptions" element={<RequirePerm perm="subscriptions:read"><Subscriptions /></RequirePerm>} />
        <Route path="/subscribers" element={<RequirePerm perm="subscriptions:read"><Subscribers /></RequirePerm>} />
        <Route path="/taxonomy" element={<RequirePerm perm="signals:read"><Taxonomy /></RequirePerm>} />
        <Route path="/jobs" element={<RequirePerm perm="jobs:read"><Jobs /></RequirePerm>} />
        <Route path="/users" element={<RequirePerm perm="users:manage"><Users /></RequirePerm>} />
        <Route path="/teams" element={<RequirePerm perm="teams:manage"><Teams /></RequirePerm>} />
        <Route path="/account" element={<Account />} />
        <Route path="/settings" element={<RequirePerm perm="settings:manage"><Settings /></RequirePerm>} />
        <Route path="/connectors" element={<RequirePerm perm="settings:manage"><Connectors /></RequirePerm>} />
        <Route path="/api-keys" element={<RequirePerm perm="settings:manage"><ApiKeys /></RequirePerm>} />
        <Route path="/audit" element={<RequirePerm perm="settings:manage"><AuditLog /></RequirePerm>} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
