import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { TopNav } from "../../features/navigation/ui/TopNav";

export function PolicyPage() {
  return (
    <main className="page">
      <TopNav />

      <section className="hero reveal reveal-2">
        <p className="eyebrow">CC Poker Wallet Rules</p>
        <h1>Wallet Policy</h1>
        <p className="subtitle">초기 칩 지급 정책과 서버 권위형 지갑 관리 원칙입니다.</p>
      </section>

      <Card className="panel controls reveal reveal-3">
        <CardHeader>
          <CardTitle>기본 정책</CardTitle>
          <CardDescription>모든 잔액은 서버 DB 기준으로만 인정됩니다.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <p>
            Guest: <Badge variant="secondary">2,000</Badge>
          </p>
          <p>
            Login + Email Verified: <Badge>10,000</Badge>
          </p>
          <p>실제 지갑 상태는 클라이언트가 아닌 백엔드/DB에서 검증됩니다.</p>
        </CardContent>
      </Card>
    </main>
  );
}
