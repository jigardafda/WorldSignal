import { Center } from "@mantine/core";
import { Navigate, Route, Routes, useLocation } from "react-router-dom";
import { Layout } from "./components/Layout";
import { LoadingState } from "./components/States";
import { useAuth } from "./lib/auth";
import { Login } from "./pages/Login";
import { Dashboard } from "./pages/Dashboard";
import { LiveDashboard } from "./pages/LiveDashboard";
import { Signals } from "./pages/Signals";
import { SignalDetail } from "./pages/SignalDetail";
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
import { AuditLog } from "./pages/AuditLog";

function RequireAuth() {
  const { user, loading } = useAuth();
  const loc = useLocation();
  if (loading) return <Center mih="100vh"><LoadingState /></Center>;
  if (!user) return <Navigate to="/login" state={{ from: loc.pathname }} replace />;
  return <Layout />;
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route element={<RequireAuth />}>
        <Route path="/" element={<Dashboard />} />
        <Route path="/live" element={<LiveDashboard />} />
        <Route path="/signals" element={<Signals />} />
        <Route path="/signals/:id" element={<SignalDetail />} />
        <Route path="/analytics" element={<Analytics />} />
        <Route path="/sources" element={<Sources />} />
        <Route path="/sources/:id" element={<SourceDetail />} />
        <Route path="/coverage" element={<Coverage />} />
        <Route path="/articles" element={<Articles />} />
        <Route path="/articles/:id" element={<ArticleDetail />} />
        <Route path="/raw-items" element={<RawItems />} />
        <Route path="/raw-items/:id" element={<RawItemDetail />} />
        <Route path="/deliveries" element={<Deliveries />} />
        <Route path="/deliveries/:id" element={<DeliveryDetail />} />
        <Route path="/subscriptions" element={<Subscriptions />} />
        <Route path="/subscribers" element={<Subscribers />} />
        <Route path="/taxonomy" element={<Taxonomy />} />
        <Route path="/jobs" element={<Jobs />} />
        <Route path="/users" element={<Users />} />
        <Route path="/teams" element={<Teams />} />
        <Route path="/account" element={<Account />} />
        <Route path="/settings" element={<Settings />} />
        <Route path="/audit" element={<AuditLog />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
