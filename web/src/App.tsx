import { Navigate, Outlet, Route, Routes } from "react-router-dom";
import { useSession } from "./session";
import { AppShell } from "./components/AppShell";
import { Login } from "./routes/Login";
import { Dashboard } from "./routes/Dashboard";
import { AddPartner } from "./routes/AddPartner";
import { PartnerDetail } from "./routes/PartnerDetail";
import { BulkImport } from "./routes/BulkImport";
import { PublicOnboarding } from "./routes/PublicOnboarding";

export default function App() {
  const { state, refresh, logout } = useSession();

  return (
    <Routes>
      {/* Public routes */}
      <Route path="/onboard" element={<PublicOnboarding />} />
      <Route
        path="/login"
        element={
          state.status === "authenticated" ? (
            <Navigate to="/dashboard" replace />
          ) : (
            <Login state={state} onRetry={refresh} />
          )
        }
      />

      {/* Authenticated admin routes */}
      <Route element={<ProtectedLayout />}>
        <Route index element={<Navigate to="/dashboard" replace />} />
        <Route path="/dashboard" element={<Dashboard />} />
        <Route path="/partners/new" element={<AddPartner />} />
        <Route path="/partners/:id" element={<PartnerDetail />} />
        <Route path="/import" element={<BulkImport />} />
        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Route>
    </Routes>
  );

  function ProtectedLayout() {
    if (state.status === "loading") {
      return (
        <main className="shell">
          <section className="panel">
            <p className="muted">Checking session…</p>
          </section>
        </main>
      );
    }
    if (state.status === "anonymous") {
      return <Navigate to="/login" replace />;
    }
    if (state.status === "error") {
      return <Login state={state} onRetry={refresh} />;
    }
    return (
      <AppShell user={state.user} onLogout={logout}>
        <Outlet />
      </AppShell>
    );
  }
}
