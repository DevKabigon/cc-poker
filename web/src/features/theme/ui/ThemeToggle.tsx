import { Monitor, Moon, Sun } from "lucide-react";
import { useTheme } from "next-themes";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";

export function ThemeToggle() {
  const { theme, setTheme } = useTheme();
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  const activeTheme = mounted ? theme ?? "system" : "system";

  return (
    <div className="theme-toggle" role="group" aria-label="Theme selector">
      <Button
        type="button"
        size="sm"
        variant={activeTheme === "light" ? "default" : "outline"}
        onClick={() => setTheme("light")}
        aria-label="Light mode"
      >
        <Sun className="size-4" />
      </Button>
      <Button
        type="button"
        size="sm"
        variant={activeTheme === "dark" ? "default" : "outline"}
        onClick={() => setTheme("dark")}
        aria-label="Dark mode"
      >
        <Moon className="size-4" />
      </Button>
      <Button
        type="button"
        size="sm"
        variant={activeTheme === "system" ? "default" : "outline"}
        onClick={() => setTheme("system")}
        aria-label="System mode"
      >
        <Monitor className="size-4" />
      </Button>
    </div>
  );
}
