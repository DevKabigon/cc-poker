import { useEffect } from "react";
import { Link, Navigate, useParams } from "react-router-dom";
import { TopNav } from "../../features/navigation/ui/TopNav";
import { StatusBadge } from "../../features/play/ui/StatusBadge";
import { usePlayConsole } from "../../features/play/model/usePlayConsole";
import { findRoomById, parseRoomIDFromTableID } from "../../features/rooms/model/roomCatalog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

export function TablePage() {
  const { tableId = "" } = useParams();
  const roomID = parseRoomIDFromTableID(tableId);
  const room = roomID ? findRoomById(roomID) : null;

  const {
    selectedTable,
    setSelectedTable,
    buyInAmount,
    setBuyInAmount,
    status,
    health,
    wallet,
    walletError,
    walletLoading,
    session,
    notice,
    lastError,
    events,
    seatGrid,
    logout,
    connectSocket,
    requestBuyIn,
    joinTable,
    leaveTable
  } = usePlayConsole();

  useEffect(() => {
    if (!tableId) {
      return;
    }
    setSelectedTable(tableId);
  }, [setSelectedTable, tableId]);

  if (!session) {
    return <Navigate to="/auth" replace />;
  }

  if (!room || tableId === "") {
    return (
      <main className="page">
        <TopNav />
        <Alert className="reveal reveal-3">
          <AlertTitle>유효하지 않은 테이블 ID입니다</AlertTitle>
          <AlertDescription>
            테이블 목록에서 다시 선택해주세요. <Link to="/lobby">로비로 이동</Link>
          </AlertDescription>
        </Alert>
      </main>
    );
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
        <p className="eyebrow">CC Poker Table</p>
        <h1>{`${room.label} - ${tableId}`}</h1>
        <p className="subtitle">
          바이인 후 소켓 접속 -&gt; 입장 -&gt; 퇴장을 검증합니다. 룸 이동은 상단 내비게이션으로 가능합니다.
        </p>
      </section>

      <Card className="panel controls reveal reveal-3">
        <CardHeader>
          <CardTitle>테이블 제어</CardTitle>
          <CardDescription>
            {`Room ${room.id} / Table ${tableId} 기준으로 바이인과 실시간 이벤트를 수행합니다.`}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="status-grid">
            <StatusBadge label="Room" value={room.label} />
            <StatusBadge label="Table" value={selectedTable} />
            <StatusBadge label="Session" value={session.player_id} />
            <StatusBadge label="WS" value={status} />
            <StatusBadge label="Backend" value={health?.status ?? "unknown"} />
            <StatusBadge label="Wallet" value={walletValue} />
          </div>

          <div className="field" style={{ maxWidth: "100%" }}>
            <Label htmlFor="buyin-amount">Buy-In Amount</Label>
            <Input
              id="buyin-amount"
              type="number"
              min={1}
              value={buyInAmount}
              onChange={(event) => setBuyInAmount(event.target.value)}
            />
            <p>
              Allowed Range:{" "}
              <Badge variant="outline">
                {formatChips(room.minBuyIn)} - {formatChips(room.maxBuyIn)}
              </Badge>
            </p>
          </div>

          <div className="button-row" style={{ marginTop: 0 }}>
            <Button variant="outline" onClick={connectSocket}>
              Connect WS
            </Button>
            <Button variant="outline" onClick={requestBuyIn}>
              Buy-In
            </Button>
            <Button variant="outline" onClick={joinTable} disabled={status !== "connected"}>
              Join Table
            </Button>
            <Button variant="outline" onClick={leaveTable} disabled={status !== "connected"}>
              Leave Table
            </Button>
            <Button variant="outline" onClick={logout}>
              Logout
            </Button>
            <Button asChild variant="outline">
              <Link to={`/rooms/${room.id}`}>룸 목록</Link>
            </Button>
          </div>

          {notice && (
            <Alert>
              <AlertTitle>{notice.kind === "success" ? "완료" : "안내"}</AlertTitle>
              <AlertDescription>{notice.message}</AlertDescription>
            </Alert>
          )}

          {lastError && (
            <Alert variant="destructive">
              <AlertTitle>오류</AlertTitle>
              <AlertDescription>{lastError}</AlertDescription>
            </Alert>
          )}
        </CardContent>
      </Card>

      <section className="layout">
        <Card className="panel table-panel reveal reveal-4">
          <CardHeader>
            <CardTitle>Table Seat Map</CardTitle>
            <CardDescription>현재 테이블 스냅샷 좌석 정보</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="seat-map">
              {seatGrid.map((slot) => (
                <div key={slot.seat} className={`seat ${slot.player ? "occupied" : ""}`}>
                  <span className="seat-num">Seat {slot.seat}</span>
                  <strong>
                    {slot.player ? (
                      <>
                        {slot.player.nickname} <Badge variant="secondary">{slot.player.stack}</Badge>
                      </>
                    ) : (
                      "Empty"
                    )}
                  </strong>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>

        <Card className="panel event-panel reveal reveal-4">
          <CardHeader>
            <CardTitle>Recent Events</CardTitle>
            <CardDescription>최근 12개 이벤트 로그</CardDescription>
          </CardHeader>
          <CardContent>
            <ul>
              {events.map((event, index) => (
                <li key={`${event}-${index}`}>{event}</li>
              ))}
            </ul>
          </CardContent>
        </Card>
      </section>
    </main>
  );
}

function formatChips(value: number) {
  return `$${value.toLocaleString("en-US")}`;
}
