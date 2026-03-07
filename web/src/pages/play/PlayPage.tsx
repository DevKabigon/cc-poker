import { Link } from "react-router-dom";
import { TopNav } from "../../features/navigation/ui/TopNav";
import { StatusBadge } from "../../features/play/ui/StatusBadge";
import { usePlayConsole } from "../../features/play/model/usePlayConsole";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";

export function PlayPage() {
  const {
    selectedTable,
    setSelectedTable,
    buyInAmount,
    setBuyInAmount,
    status,
    health,
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

  return (
    <main className="page">
      <TopNav />

      <section className="hero">
        <p className="eyebrow">CC Poker Play Console</p>
        <h1>Socket + Buy-In + Join / Leave</h1>
        <p className="subtitle">
          인증 이후 소켓 연결, 바이인, 테이블 입장/퇴장을 단계별로 검증합니다.
        </p>
      </section>

      {!session && (
        <Alert>
          <AlertTitle>세션이 필요합니다</AlertTitle>
          <AlertDescription>
            이 페이지를 사용하려면 먼저 인증이 필요합니다. <Link to="/auth">/auth</Link> 에서 게스트
            또는 로그인 세션을 생성하세요.
          </AlertDescription>
        </Alert>
      )}

      <Card className="panel controls" style={{ marginTop: 16 }}>
        <CardHeader>
          <CardTitle>테이블 제어</CardTitle>
          <CardDescription>현재 선택 테이블 기준으로 바이인 및 실시간 액션을 요청합니다.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="field" style={{ maxWidth: "100%" }}>
            <Label htmlFor="table-id">Table ID</Label>
            <Input
              id="table-id"
              value={selectedTable}
              onChange={(event) => setSelectedTable(event.target.value)}
              maxLength={40}
            />
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
          </div>

          <div className="button-row" style={{ marginTop: 0 }}>
            <Button variant="outline" onClick={connectSocket} disabled={!session}>
              Connect WS
            </Button>
            <Button variant="outline" onClick={requestBuyIn} disabled={!session}>
              Buy-In
            </Button>
            <Button variant="outline" onClick={joinTable} disabled={status !== "connected"}>
              Join Table
            </Button>
            <Button variant="outline" onClick={leaveTable} disabled={status !== "connected"}>
              Leave Table
            </Button>
            <Button variant="outline" onClick={logout} disabled={!session}>
              Logout
            </Button>
          </div>

          <div className="status-grid">
            <StatusBadge label="Session" value={session ? session.player_id : "none"} />
            <StatusBadge label="Nickname" value={session ? session.nickname : "-"} />
            <StatusBadge label="WS" value={status} />
            <StatusBadge label="Backend" value={health?.status ?? "unknown"} />
            <StatusBadge
              label="Players"
              value={`${seatGrid.filter((slot) => slot.player !== null).length}/9`}
            />
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
        <Card className="panel table-panel">
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

        <Card className="panel event-panel">
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
