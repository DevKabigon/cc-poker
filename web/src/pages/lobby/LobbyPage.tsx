import { Link, Navigate } from "react-router-dom";
import { TopNav } from "../../features/navigation/ui/TopNav";
import { usePlayConsole } from "../../features/play/model/usePlayConsole";
import { ROOM_CATALOG } from "../../features/rooms/model/roomCatalog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

export function LobbyPage() {
  const { session, wallet, walletError, walletLoading } = usePlayConsole();

  if (!session) {
    return <Navigate to="/auth" replace />;
  }

  const walletValue = walletLoading
    ? "loading..."
    : walletError
      ? "unavailable"
      : wallet
        ? formatChips(wallet.balance)
        : "-";

  return (
    <main className="page">
      <TopNav />

      <section className="hero reveal reveal-2">
        <p className="eyebrow">CC Poker Lobby</p>
        <h1>블라인드 룸을 선택하세요</h1>
        <p className="subtitle">
          로비에서 블라인드 레벨을 고른 뒤, 룸 페이지에서 원하는 테이블로 입장합니다.
        </p>
      </section>

      <Card className="panel controls reveal reveal-3">
        <CardHeader>
          <CardTitle>현재 세션</CardTitle>
          <CardDescription>쿠키 기반 세션이 유지되면 새로고침 이후에도 자동 복구됩니다.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <p>
            Player: <Badge variant="outline">{session.nickname}</Badge>
          </p>
          <p>
            Wallet: <Badge variant="outline">{walletValue}</Badge>
          </p>
          {walletError && (
            <Alert variant="destructive">
              <AlertTitle>지갑 조회 실패</AlertTitle>
              <AlertDescription>{walletError}</AlertDescription>
            </Alert>
          )}
        </CardContent>
      </Card>

      <section className="room-grid reveal reveal-4">
        {ROOM_CATALOG.map((room) => (
          <Card key={room.id} className="panel room-card">
            <CardHeader>
              <CardTitle>{room.label} Room</CardTitle>
              <CardDescription>{`Blinds ${room.smallBlind}/${room.bigBlind}`}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <p>
                Buy-In:{" "}
                <Badge variant="outline">
                  {formatChips(room.minBuyIn)} - {formatChips(room.maxBuyIn)}
                </Badge>
              </p>
              <p>
                Tables: <Badge variant="outline">{room.tableCount}</Badge>
              </p>
              <p>
                Max Players: <Badge variant="outline">{room.maxPlayers}</Badge>
              </p>
              <Button asChild className="w-full">
                <Link to={`/rooms/${room.id}`}>룸 입장</Link>
              </Button>
            </CardContent>
          </Card>
        ))}
      </section>
    </main>
  );
}

function formatChips(value: number) {
  return `$${value.toLocaleString("en-US")}`;
}
