import { useCallback, useMemo, useRef, useState } from "react";

type SessionResponse = {
  player_id: string;
  nickname: string;
  expires_at: string;
};

type SnapshotPlayer = {
  player_id: string;
  nickname: string;
  seat_index: number;
};

type TableSnapshot = {
  table_state: string;
  max_players: number;
  min_players_to_start: number;
  active_players: number;
  can_start: boolean;
  players: SnapshotPlayer[];
};

type ServerEnvelope = {
  event_type: string;
  table_id?: string;
  payload: unknown;
  sent_at: string;
};

type ConnectionStatus = "idle" | "connecting" | "connected" | "closed" | "error";

const DEFAULT_TABLE = "table-1";

export default function App() {
  const [nickname, setNickname] = useState("Kabigon");
  const [session, setSession] = useState<SessionResponse | null>(null);
  const [status, setStatus] = useState<ConnectionStatus>("idle");
  const [snapshot, setSnapshot] = useState<TableSnapshot | null>(null);
  const [events, setEvents] = useState<string[]>([]);
  const [lastError, setLastError] = useState<string>("");
  const wsRef = useRef<WebSocket | null>(null);

  const appendEvent = useCallback((next: string) => {
    setEvents((prev) => [next, ...prev].slice(0, 12));
  }, []);

  const createGuestSession = useCallback(async () => {
    setLastError("");
    try {
      const response = await fetch("/v1/session/guest", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ nickname })
      });

      if (!response.ok) {
        throw new Error(`guest session failed: ${response.status}`);
      }

      const data = (await response.json()) as SessionResponse;
      setSession(data);
      appendEvent(`SESSION ${data.player_id} (${data.nickname})`);
    } catch (error) {
      const message = error instanceof Error ? error.message : "unknown session error";
      setLastError(message);
      appendEvent(`SESSION_ERROR ${message}`);
    }
  }, [appendEvent, nickname]);

  const closeSocket = useCallback(() => {
    const ws = wsRef.current;
    if (!ws) {
      return;
    }
    ws.close();
    wsRef.current = null;
  }, []);

  const connectSocket = useCallback(() => {
    setLastError("");
    closeSocket();

    const protocol = window.location.protocol === "https:" ? "wss" : "ws";
    const ws = new WebSocket(`${protocol}://${window.location.host}/ws`);
    wsRef.current = ws;
    setStatus("connecting");

    ws.onopen = () => {
      setStatus("connected");
      appendEvent("WS OPEN");
    };

    ws.onmessage = (event) => {
      try {
        const parsed = JSON.parse(event.data) as ServerEnvelope;
        if (parsed.event_type === "table_snapshot") {
          setSnapshot(parsed.payload as TableSnapshot);
        }
        appendEvent(`EVENT ${parsed.event_type}`);
      } catch {
        appendEvent("EVENT_PARSE_ERROR");
      }
    };

    ws.onclose = () => {
      setStatus("closed");
      appendEvent("WS CLOSE");
      wsRef.current = null;
    };

    ws.onerror = () => {
      setStatus("error");
      setLastError("websocket error");
      appendEvent("WS ERROR");
    };
  }, [appendEvent, closeSocket]);

  const sendEvent = useCallback(
    (eventType: string, payload: Record<string, unknown>) => {
      const ws = wsRef.current;
      if (!ws || ws.readyState !== WebSocket.OPEN) {
        setLastError("websocket is not connected");
        appendEvent("SEND_SKIPPED_NOT_CONNECTED");
        return;
      }

      ws.send(
        JSON.stringify({
          event_type: eventType,
          request_id: createRequestId(),
          payload
        })
      );
      appendEvent(`SEND ${eventType}`);
    },
    [appendEvent]
  );

  const joinTable = useCallback(() => {
    sendEvent("join_table", { table_id: DEFAULT_TABLE });
  }, [sendEvent]);

  const leaveTable = useCallback(() => {
    sendEvent("leave_table", { table_id: DEFAULT_TABLE });
  }, [sendEvent]);

  const seatGrid = useMemo(() => {
    const occupied = new Map<number, SnapshotPlayer>();
    snapshot?.players.forEach((player) => occupied.set(player.seat_index, player));

    return Array.from({ length: 9 }).map((_, index) => ({
      seat: index + 1,
      player: occupied.get(index) ?? null
    }));
  }, [snapshot]);

  return (
    <main className="page">
      <section className="hero">
        <p className="eyebrow">CC Poker Developer Console</p>
        <h1>Session + WebSocket + Table Snapshot</h1>
        <p className="subtitle">
          현재 단계는 서버 세로 슬라이스 검증용 UI입니다. 게스트 세션 발급 후 WS를 연결하고
          join/leave 이벤트가 실시간으로 반영되는지 확인할 수 있습니다.
        </p>
      </section>

      <section className="panel controls">
        <div className="field">
          <label htmlFor="nickname">Nickname</label>
          <input
            id="nickname"
            value={nickname}
            onChange={(event) => setNickname(event.target.value)}
            maxLength={20}
          />
        </div>

        <div className="button-row">
          <button onClick={createGuestSession}>Create Guest Session</button>
          <button
            className="outline"
            onClick={connectSocket}
            disabled={!session}
            title={session ? "connect websocket" : "create session first"}
          >
            Connect WS
          </button>
          <button className="outline" onClick={joinTable} disabled={status !== "connected"}>
            Join Table
          </button>
          <button className="outline" onClick={leaveTable} disabled={status !== "connected"}>
            Leave Table
          </button>
        </div>

        <div className="status-grid">
          <StatusBadge label="Session" value={session ? session.player_id : "none"} />
          <StatusBadge label="WS" value={status} />
          <StatusBadge label="Table" value={snapshot?.table_state ?? "unknown"} />
          <StatusBadge
            label="Players"
            value={`${snapshot?.active_players ?? 0}/${snapshot?.max_players ?? 9}`}
          />
        </div>

        {lastError && <p className="error-text">ERROR: {lastError}</p>}
      </section>

      <section className="layout">
        <article className="panel table-panel">
          <h2>Table Seat Map</h2>
          <div className="seat-map">
            {seatGrid.map((slot) => (
              <div key={slot.seat} className={`seat ${slot.player ? "occupied" : ""}`}>
                <span className="seat-num">Seat {slot.seat}</span>
                <strong>{slot.player ? slot.player.nickname : "Empty"}</strong>
              </div>
            ))}
          </div>
        </article>

        <article className="panel event-panel">
          <h2>Recent Events</h2>
          <ul>
            {events.map((event, index) => (
              <li key={`${event}-${index}`}>{event}</li>
            ))}
          </ul>
        </article>
      </section>
    </main>
  );
}

function createRequestId() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return `req_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
}

function StatusBadge({ label, value }: { label: string; value: string }) {
  return (
    <div className="status-card">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}
