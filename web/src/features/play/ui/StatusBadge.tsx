import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";

type StatusBadgeProps = {
  label: string;
  value: string;
};

export function StatusBadge({ label, value }: StatusBadgeProps) {
  return (
    <Card className="status-card">
      <CardContent className="p-3">
        <span>{label}</span>
        <div>
          <Badge variant="outline" className="max-w-full break-all">
            {value}
          </Badge>
        </div>
      </CardContent>
    </Card>
  );
}
