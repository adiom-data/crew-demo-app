import type { ReactNode } from "react";
import { NavLink } from "react-router-dom";
import type { User } from "../gen/sample/v1/sample_pb";

interface AppShellProps {
  user?: User;
  onLogout: () => void;
  children: ReactNode;
}

const NAV = [
  { to: "/dashboard", label: "Dashboard" },
  { to: "/partners/new", label: "Add Partner" },
  { to: "/import", label: "Bulk Import" },
];

export function AppShell({ user, onLogout, children }: AppShellProps) {
  return (
    <div className="app">
      <header className="topnav">
        <div className="topnav-inner">
          <div className="brand">
            <span className="brand-mark">◇</span>
            <span className="brand-name">On-board</span>
          </div>
          <nav className="navlinks">
            {NAV.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                className={({ isActive }: { isActive: boolean }) => (isActive ? "nav-link active" : "nav-link")}
                end={item.to === "/dashboard"}
              >
                {item.label}
              </NavLink>
            ))}
          </nav>
          <div className="topnav-user">
            <span className="topnav-avatar">{initials(user)}</span>
            <div className="topnav-identity">
              <span className="topnav-name">{user?.name || user?.email || "Admin"}</span>
              <span className="topnav-email">{user?.email}</span>
            </div>
            <button className="button danger" onClick={onLogout}>
              Logout
            </button>
          </div>
        </div>
      </header>
      <main className="content">{children}</main>
    </div>
  );
}

function initials(user?: User): string {
  const source = user?.name || user?.email || "?";
  return source
    .split(/\s+/)
    .map((part) => part[0])
    .join("")
    .slice(0, 2)
    .toUpperCase();
}
