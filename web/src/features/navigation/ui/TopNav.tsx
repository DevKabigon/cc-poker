import { NavLink, useLocation } from "react-router-dom";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

export function TopNav() {
  const location = useLocation();

  return (
    <nav className="panel controls" style={{ marginBottom: 12, display: "flex", gap: 12 }}>
      <Badge variant="secondary">CC Poker</Badge>
      <div className="button-row" style={{ marginTop: 0 }}>
        <Button asChild variant={location.pathname === "/auth" ? "default" : "outline"} size="sm">
          <NavLink to="/auth">Auth</NavLink>
        </Button>
        <Button asChild variant={location.pathname === "/play" ? "default" : "outline"} size="sm">
          <NavLink to="/play">Play Console</NavLink>
        </Button>
        <Button
          asChild
          variant={location.pathname === "/policy" ? "default" : "outline"}
          size="sm"
        >
          <NavLink to="/policy">Wallet Policy</NavLink>
        </Button>
      </div>
    </nav>
  );
}
