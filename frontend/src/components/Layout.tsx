import { AppShell, Burger, Group, Menu, NavLink, ScrollArea, SegmentedControl, Text, Avatar, UnstyledButton } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import {
  IconActivity, IconArticle, IconBell, IconBroadcast, IconChartBar, IconDatabase,
  IconFileText, IconGauge, IconListCheck, IconLogout, IconMail, IconSettings, IconSitemap,
  IconUsers, IconUsersGroup,
} from "@tabler/icons-react";
import { NavLink as RouterLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../lib/auth";
import { LogoMark } from "./Logo";

interface NavItem { to: string; label: string; icon: React.ReactNode; perm?: string }

const NAV: NavItem[] = [
  { to: "/", label: "Dashboard", icon: <IconGauge size={18} /> },
  { to: "/signals", label: "Signals", icon: <IconActivity size={18} />, perm: "signals:read" },
  { to: "/analytics", label: "Analytics", icon: <IconChartBar size={18} />, perm: "analytics:read" },
  { to: "/sources", label: "Sources", icon: <IconBroadcast size={18} />, perm: "sources:read" },
  { to: "/coverage", label: "Coverage", icon: <IconChartBar size={18} />, perm: "sources:read" },
  { to: "/articles", label: "Articles", icon: <IconArticle size={18} />, perm: "signals:read" },
  { to: "/raw-items", label: "Raw Items", icon: <IconFileText size={18} />, perm: "signals:read" },
  { to: "/subscriptions", label: "Subscriptions", icon: <IconBell size={18} />, perm: "subscriptions:read" },
  { to: "/subscribers", label: "Subscribers", icon: <IconUsersGroup size={18} />, perm: "subscriptions:read" },
  { to: "/deliveries", label: "Deliveries", icon: <IconDatabase size={18} />, perm: "deliveries:read" },
  { to: "/taxonomy", label: "Taxonomy", icon: <IconSitemap size={18} />, perm: "signals:read" },
  { to: "/jobs", label: "Jobs", icon: <IconListCheck size={18} />, perm: "jobs:read" },
  { to: "/users", label: "Users", icon: <IconUsers size={18} />, perm: "users:manage" },
  { to: "/teams", label: "Teams", icon: <IconUsersGroup size={18} />, perm: "teams:manage" },
  { to: "/settings", label: "Settings", icon: <IconSettings size={18} />, perm: "settings:manage" },
  { to: "/connectors", label: "Connectors", icon: <IconMail size={18} />, perm: "settings:manage" },
  { to: "/audit", label: "Audit Log", icon: <IconListCheck size={18} />, perm: "settings:manage" },
];

export function Layout() {
  const [opened, { toggle }] = useDisclosure();
  const { user, logout, hasPerm } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const live = location.pathname === "/live"; // URL-driven so it survives reloads

  const visible = NAV.filter((n) => !n.perm || hasPerm(n.perm));

  return (
    <AppShell
      header={{ height: 56 }}
      navbar={{ width: 240, breakpoint: "sm", collapsed: { mobile: !opened || live, desktop: live } }}
      padding={live ? 0 : "md"}
    >
      <AppShell.Header>
        <Group h="100%" px="md" justify="space-between" wrap="nowrap">
          <Group gap="xs">
            <Burger opened={opened} onClick={toggle} hiddenFrom="sm" size="sm" />
            <LogoMark size={28} />
            <Text fw={700}>World<Text span c="blue" inherit>Signal</Text></Text>
          </Group>
          <SegmentedControl
            size="xs"
            value={live ? "live" : "dashboard"}
            onChange={(v) => navigate(v === "live" ? "/live" : "/")}
            data-testid="dashboard-mode"
            data={[
              { value: "dashboard", label: "Dashboard" },
              { value: "live", label: "Live" },
            ]}
          />
          <Menu position="bottom-end" withArrow>
            <Menu.Target>
              <UnstyledButton data-testid="user-menu">
                <Group gap="xs">
                  <Avatar color="blue" radius="xl" size={30}>{user?.email?.[0]?.toUpperCase() ?? "?"}</Avatar>
                  <div style={{ lineHeight: 1.1 }}>
                    <Text size="sm" fw={600}>{user?.name || user?.email}</Text>
                    <Text size="xs" c="dimmed">{user?.role}</Text>
                  </div>
                </Group>
              </UnstyledButton>
            </Menu.Target>
            <Menu.Dropdown>
              <Menu.Item leftSection={<IconSettings size={16} />} onClick={() => navigate("/account")}>Account</Menu.Item>
              <Menu.Item color="red" leftSection={<IconLogout size={16} />} onClick={() => { void logout(); }}>Log out</Menu.Item>
            </Menu.Dropdown>
          </Menu>
        </Group>
      </AppShell.Header>

      <AppShell.Navbar p="xs">
        <ScrollArea>
          {visible.map((n) => (
            <NavLink
              key={n.to}
              component={RouterLink}
              to={n.to}
              label={n.label}
              leftSection={n.icon}
              active={n.to === "/" ? location.pathname === "/" : location.pathname.startsWith(n.to)}
              onClick={() => opened && toggle()}
            />
          ))}
        </ScrollArea>
      </AppShell.Navbar>

      <AppShell.Main>
        <Outlet />
      </AppShell.Main>
    </AppShell>
  );
}
