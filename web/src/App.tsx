import { useCallback, useMemo, useRef, useState } from "react";
import useSWR from "swr";
import { NavLink, Navigate, Route, Routes } from "react-router-dom";
import { useAppStore, type SessionSnapshot } from "./store/useAppStore";

type SnapshotPlayer = {
  player_id: string;
  nickname: string;
  seat_index: number;
  stack: number;
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

type SupabaseLoginResponse = {
  access_token: string;
};

type HealthResponse = {
  status: string;
  service: string;
  timestamp: string;
};

type ConnectionStatus = "idle" | "connecting" | "connected" | "closed" | "error";

const DEFAULT_TABLE = "room_1_2_table_1";
const SUPABASE_URL = String(import.meta.env.VITE_SUPABASE_URL ?? "").trim();
const SUPABASE_ANON_KEY = String(import.meta.env.VITE_SUPABASE_ANON_KEY ?? "").trim();
const fetcher = (url: string) => fetch(url, { credentials: "include" }).then((res) => res.json());

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/play" replace />} />
      <Route path="/play" element={<PlayPage />} />
      <Route path="/policy" element={<PolicyPage />} />
      <Route path="*" element={<Navigate to="/play" replace />} />
    </Routes>
  );
}

function PlayPage() {
  const [nickname, setNickname] = useState("Kabigon");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [status, setStatus] = useState<ConnectionStatus>("idle");
  const [snapshot, setSnapshot] = useState<TableSnapshot | null>(null);
  const [events, setEvents] = useState<string[]>([]);
  const [selectedTable, setSelectedTable] = useState(DEFAULT_TABLE);
  const [buyInAmount, setBuyInAmount] = useState("200");
  const wsRef = useRef<WebSocket | null>(null);

  const session = useAppStore((state) => state.session);
  const setSession = useAppStore((state) => state.setSession);
  const clearSession = useAppStore((state) => state.clearSession);
  const lastError = useAppStore((state) => state.lastError);
  const setLastError = useAppStore((state) => state.setLastError);
  const clearLastError = useAppStore((state) => state.clearLastError);

  const { data: health } = useSWR<HealthResponse>("/health", fetcher, {
    refreshInterval: 15000,
    dedupingInterval: 5000
  });

  const appendEvent = useCallback((next: string) => {
    setEvents((prev) => [next, ...prev].slice(0, 12));
  }, []);

  const applySession = useCallback(
    (nextSession: SessionSnapshot) => {
      setSession(nextSession);
      appendEvent(`SESSION ${nextSession.player_id} (${nextSession.nickname})`);
    },
    [appendEvent, setSession]
  );

  const requireSupabaseConfig = useCallback(() => {
    if (SUPABASE_URL === "" || SUPABASE_ANON_KEY === "") {
      throw new Error("missing VITE_SUPABASE_URL or VITE_SUPABASE_ANON_KEY");
    }
  }, []);

  const exchangeAccessToken = useCallback(
    async (accessToken: string) => {
      const response = await fetch("/v1/auth/exchange", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({
          access_token: accessToken,
          nickname
        })
      });
      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`auth exchange failed: ${response.status} ${errorText}`);
      }

      const data = (await response.json()) as SessionSnapshot;
      applySession(data);
      appendEvent("AUTH_EXCHANGE_OK");
    },
    [applySession, appendEvent, nickname]
  );

  const createGuestSession = useCallback(async () => {
    clearLastError();
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

      const data = (await response.json()) as SessionSnapshot;
      applySession(data);
      appendEvent("GUEST_OK");
    } catch (error) {
      const message = error instanceof Error ? error.message : "unknown session error";
      setLastError(message);
      appendEvent(`GUEST_ERROR ${message}`);
    }
  }, [applySession, appendEvent, clearLastError, nickname, setLastError]);

  const signUpWithSupabase = useCallback(async () => {
    clearLastError();
    try {
      requireSupabaseConfig();
      if (!email.trim() || !password.trim()) {
        throw new Error("email and password are required");
      }

      const response = await fetch(`${SUPABASE_URL}/auth/v1/signup`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          apikey: SUPABASE_ANON_KEY
        },
        body: JSON.stringify({
          email: email.trim(),
          password,
          data: { nickname }
        })
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`signup failed: ${response.status} ${errorText}`);
      }

      appendEvent("SIGNUP_OK verify your email");
    } catch (error) {
      const message = error instanceof Error ? error.message : "unknown signup error";
      setLastError(message);
      appendEvent(`SIGNUP_ERROR ${message}`);
    }
  }, [
    appendEvent,
    clearLastError,
    email,
    nickname,
    password,
    requireSupabaseConfig,
    setLastError
  ]);

  const signInWithSupabase = useCallback(async () => {
    clearLastError();
    try {
      requireSupabaseConfig();
      if (!email.trim() || !password.trim()) {
        throw new Error("email and password are required");
      }

      const response = await fetch(`${SUPABASE_URL}/auth/v1/token?grant_type=password`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          apikey: SUPABASE_ANON_KEY
        },
        body: JSON.stringify({
          email: email.trim(),
          password
        })
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`signin failed: ${response.status} ${errorText}`);
      }

      const payload = (await response.json()) as SupabaseLoginResponse;
      await exchangeAccessToken(payload.access_token);
      appendEvent("SIGNIN_OK");
    } catch (error) {
      const message = error instanceof Error ? error.message : "unknown signin error";
      setLastError(message);
      appendEvent(`SIGNIN_ERROR ${message}`);
    }
  }, [
    appendEvent,
    clearLastError,
    email,
    exchangeAccessToken,
    password,
    requireSupabaseConfig,
    setLastError
  ]);

  const closeSocket = useCallback(() => {
    const ws = wsRef.current;
    if (!ws) {
      return;
    }
    ws.close();
    wsRef.current = null;
  }, []);

  const logout = useCallback(async () => {
    clearLastError();
    closeSocket();
    setStatus("idle");
    setSnapshot(null);
    try {
      await fetch("/v1/auth/logout", {
        method: "POST",
        credentials: "include"
      });
    } finally {
      clearSession();
      appendEvent("LOGOUT");
    }
  }, [appendEvent, clearLastError, clearSession, closeSocket]);

  const connectSocket = useCallback(() => {
    clearLastError();
    closeSocket();

    const protocol = window.location.protocol === "https:" ? "wss" : "ws";
    const ws = new WebSocket(`${protocol}://${window.location.host}/ws`);
    wsRef.current = ws;
    setStatus("connecting");

    ws.onopen = () => {
      setStatus("connected");
      appendEvent("WS_OPEN");
    };

    ws.onmessage = (event) => {
      try {
        const parsed = JSON.parse(event.data) as ServerEnvelope;
        if (parsed.event_type === "table_snapshot") {
          setSnapshot(parsed.payload as TableSnapshot);
        }
        if (parsed.event_type === "error_notice") {
          const notice = parsed.payload as { code?: string; message?: string };
          const message = notice.message ?? "unknown websocket error";
          setLastError(message);
          appendEvent(`WS_NOTICE ${notice.code ?? "UNKNOWN"} ${message}`);
          return;
        }
        appendEvent(`WS_EVENT ${parsed.event_type}`);
      } catch {
        appendEvent("WS_EVENT_PARSE_ERROR");
      }
    };

    ws.onclose = () => {
      setStatus("closed");
      appendEvent("WS_CLOSE");
      wsRef.current = null;
    };

    ws.onerror = () => {
      setStatus("error");
      setLastError("websocket error");
      appendEvent("WS_ERROR");
    };
  }, [appendEvent, clearLastError, closeSocket, setLastError]);

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
    [appendEvent, setLastError]
  );

  const joinTable = useCallback(() => {
    sendEvent("join_table", { table_id: selectedTable });
  }, [selectedTable, sendEvent]);

  const leaveTable = useCallback(() => {
    sendEvent("leave_table", { table_id: selectedTable });
  }, [selectedTable, sendEvent]);

  const requestBuyIn = useCallback(async () => {
    clearLastError();
    const parsedAmount = Number(buyInAmount);
    if (!Number.isFinite(parsedAmount) || parsedAmount <= 0) {
      setLastError("buy-in amount must be positive");
      return;
    }

    try {
      const response = await fetch("/v1/tables/buy-in", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ table_id: selectedTable, amount: Math.floor(parsedAmount) })
      });
      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`buy-in failed: ${response.status} ${errorText}`);
      }

      appendEvent(`BUY_IN ${selectedTable} ${Math.floor(parsedAmount)}`);
    } catch (error) {
      const message = error instanceof Error ? error.message : "unknown buy-in error";
      setLastError(message);
      appendEvent(`BUY_IN_ERROR ${message}`);
    }
  }, [appendEvent, buyInAmount, clearLastError, selectedTable, setLastError]);

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
      <TopNav />

      <section className="hero">
        <p className="eyebrow">CC Poker Developer Console</p>
        <h1>Routing + Zustand + SWR Applied</h1>
        <p className="subtitle">
          게스트는 2,000, 인증 완료 계정은 10,000 정책을 기준으로 세션/바이인 흐름을 검증합니다.
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
        <div className="field">
          <label htmlFor="email">Email</label>
          <input
            id="email"
            type="email"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            placeholder="you@example.com"
          />
        </div>
        <div className="field">
          <label htmlFor="password">Password</label>
          <input
            id="password"
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            placeholder="min 6 chars"
          />
        </div>
        <div className="field">
          <label htmlFor="table-id">Table ID</label>
          <input
            id="table-id"
            value={selectedTable}
            onChange={(event) => setSelectedTable(event.target.value)}
            maxLength={40}
          />
        </div>
        <div className="field">
          <label htmlFor="buyin-amount">Buy-In Amount</label>
          <input
            id="buyin-amount"
            type="number"
            min={1}
            value={buyInAmount}
            onChange={(event) => setBuyInAmount(event.target.value)}
          />
        </div>

        <div className="button-row">
          <button onClick={createGuestSession}>Create Guest Session</button>
          <button className="outline" onClick={signUpWithSupabase}>
            Sign Up
          </button>
          <button className="outline" onClick={signInWithSupabase}>
            Sign In
          </button>
          <button className="outline" onClick={logout} disabled={!session}>
            Logout
          </button>
          <button
            className="outline"
            onClick={connectSocket}
            disabled={!session}
            title={session ? "connect websocket" : "create or login first"}
          >
            Connect WS
          </button>
          <button className="outline" onClick={requestBuyIn} disabled={!session}>
            Buy-In
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
          <StatusBadge label="Nickname" value={session ? session.nickname : "-"} />
          <StatusBadge label="WS" value={status} />
          <StatusBadge label="Backend" value={health?.status ?? "unknown"} />
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
                <strong>
                  {slot.player ? `${slot.player.nickname} (${slot.player.stack})` : "Empty"}
                </strong>
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

function PolicyPage() {
  return (
    <main className="page">
      <TopNav />
      <section className="panel controls">
        <h2>Wallet Policy</h2>
        <p>Guest: 초기 체험칩 2,000</p>
        <p>Login + Email Verified: 초기 10,000</p>
        <p>실제 지갑 상태는 서버 DB 기준으로만 관리됩니다.</p>
      </section>
    </main>
  );
}

function TopNav() {
  return (
    <nav className="panel controls" style={{ marginBottom: 12 }}>
      <div className="button-row">
        <NavLink to="/play">Play Console</NavLink>
        <NavLink to="/policy">Wallet Policy</NavLink>
      </div>
    </nav>
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

