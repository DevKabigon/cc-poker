import { NavLink, useLocation } from "react-router-dom";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ThemeToggle } from "@/features/theme/ui/ThemeToggle";

const NAV_ITEMS = [
  { to: "/auth", label: "Auth" },
  { to: "/play", label: "Play Console" },
  { to: "/policy", label: "Wallet Policy" }
] as const;

export function TopNav() {
  const location = useLocation();

  return (
    <nav className="panel controls top-nav reveal reveal-1">
      <div className="top-nav__left">
        <Badge variant="secondary">CC Poker Lounge</Badge>
        <div className="button-row">
          {NAV_ITEMS.map((item) => (
            <Button
              key={item.to}
              asChild
              variant={location.pathname === item.to ? "default" : "outline"}
              size="sm"
            >
              <NavLink to={item.to}>{item.label}</NavLink>
            </Button>
          ))}
        </div>
      </div>
      <ThemeToggle />
    </nav>
  );
}
