import { Link, Navigate, useParams } from "react-router-dom";
import { TopNav } from "../../features/navigation/ui/TopNav";
import { usePlayConsole } from "../../features/play/model/usePlayConsole";
import { buildTablesForRoom, findRoomById } from "../../features/rooms/model/roomCatalog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

export function RoomPage() {
  const { roomId = "" } = useParams();
  const { session } = usePlayConsole();

  if (!session) {
    return <Navigate to="/auth" replace />;
  }

  const room = findRoomById(roomId);
  if (!room) {
    return (
      <main className="page">
        <TopNav />
        <Alert className="reveal reveal-3">
          <AlertTitle>존재하지 않는 룸입니다</AlertTitle>
          <AlertDescription>
            룸 ID를 확인해주세요. <Link to="/lobby">로비로 이동</Link>
          </AlertDescription>
        </Alert>
      </main>
    );
  }

  const tables = buildTablesForRoom(room.id);

  return (
    <main className="page">
      <TopNav />

      <section className="hero reveal reveal-2">
        <p className="eyebrow">CC Poker Room</p>
        <h1>{room.label} 룸</h1>
        <p className="subtitle">
          룸 단위 블라인드 규칙은 고정이며, 각 룸에는 기본 10개 테이블이 열려 있습니다.
        </p>
      </section>

      <Card className="panel controls reveal reveal-3">
        <CardHeader>
          <CardTitle>룸 정보</CardTitle>
          <CardDescription>{`Room ID: ${room.id}`}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <p>
            Blinds:{" "}
            <Badge variant="outline">
              {room.smallBlind}/{room.bigBlind}
            </Badge>
          </p>
          <p>
            Buy-In:{" "}
            <Badge variant="outline">
              {formatChips(room.minBuyIn)} - {formatChips(room.maxBuyIn)}
            </Badge>
          </p>
          <div className="button-row">
            <Button asChild variant="outline">
              <Link to="/lobby">로비로</Link>
            </Button>
          </div>
        </CardContent>
      </Card>

      <section className="table-select-grid reveal reveal-4">
        {tables.map((table) => (
          <Card key={table.id} className="panel room-card">
            <CardHeader>
              <CardTitle>{`Table ${table.index}`}</CardTitle>
              <CardDescription>{table.id}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <p>
                Max Players: <Badge variant="outline">{room.maxPlayers}</Badge>
              </p>
              <Button asChild className="w-full">
                <Link to={`/tables/${table.id}`}>테이블 입장</Link>
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
