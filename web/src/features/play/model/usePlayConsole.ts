import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import useSWR, { useSWRConfig } from "swr";
import { useAppStore, type SessionSnapshot } from "../../../store/useAppStore";
import type {
  BuyInResponse,
  ConnectionStatus,
  HealthResponse,
  NicknameCheckResponse,
  ServerEnvelope,
  SnapshotPlayer,
  SupabaseLoginResponse,
  SupabaseSignupResponse,
  TableSnapshot,
  UiNotice,
  WalletResponse
} from "./types";

const DEFAULT_TABLE = "room_1_2_table_1";
const SUPABASE_URL = String(import.meta.env.VITE_SUPABASE_URL ?? "").trim();
const SUPABASE_ANON_KEY = String(import.meta.env.VITE_SUPABASE_ANON_KEY ?? "").trim();

export function usePlayConsole() {
  const [nickname, setNickname] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [status, setStatus] = useState<ConnectionStatus>("idle");
  const [snapshot, setSnapshot] = useState<TableSnapshot | null>(null);
  const [events, setEvents] = useState<string[]>([]);
  const [notice, setNotice] = useState<UiNotice>(null);
  const [selectedTable, setSelectedTable] = useState(DEFAULT_TABLE);
  const [buyInAmount, setBuyInAmount] = useState("200");
  const wsRef = useRef<WebSocket | null>(null);
  const selfSeatedRef = useRef(false);

  const session = useAppStore((state) => state.session);
  const setSession = useAppStore((state) => state.setSession);
  const clearSession = useAppStore((state) => state.clearSession);
  const lastError = useAppStore((state) => state.lastError);
  const setLastError = useAppStore((state) => state.setLastError);
  const clearLastError = useAppStore((state) => state.clearLastError);
  const { mutate } = useSWRConfig();

  const { data: health } = useSWR<HealthResponse>("/health", fetchJSON, {
    revalidateOnFocus: false,
    revalidateOnReconnect: false,
    shouldRetryOnError: false
  });

  const { data: restoredSession } = useSWR<SessionSnapshot | null>(
    "/v1/session/current",
    fetchCurrentSession,
    {
      revalidateOnFocus: false,
      revalidateOnReconnect: true,
      shouldRetryOnError: false
    }
  );

  useEffect(() => {
    if (!restoredSession) {
      return;
    }
    setSession(restoredSession);
  }, [restoredSession, setSession]);

  useEffect(() => {
    if (!session || !snapshot) {
      selfSeatedRef.current = false;
      return;
    }
    selfSeatedRef.current = snapshot.players.some((player) => player.player_id === session.player_id);
  }, [session, snapshot]);

  const walletKey = session ? "/v1/wallet" : null;
  const { data: wallet, error: walletError, isLoading: walletLoading } = useSWR<WalletResponse>(
    walletKey,
    fetchJSON,
    {
      revalidateOnFocus: true,
      revalidateOnReconnect: true,
      shouldRetryOnError: false
    }
  );

  const appendEvent = useCallback((next: string) => {
    setEvents((prev) => [next, ...prev].slice(0, 12));
  }, []);

  const applySession = useCallback(
    (nextSession: SessionSnapshot) => {
      setSession(nextSession);
      appendEvent(`SESSION ${nextSession.player_id} (${nextSession.nickname})`);
      void mutate("/v1/session/current", nextSession, { revalidate: false });
      void mutate("/v1/wallet");
    },
    [appendEvent, mutate, setSession]
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
      if (!nickname.trim()) {
        throw new Error("nickname is required");
      }
      const response = await fetch("/v1/session/guest", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ nickname })
      });

      if (!response.ok) {
        const errorText = (await response.text()).trim().toLowerCase();
        if (response.status === 409 || errorText.includes("nickname is already taken")) {
          throw new Error("이미 사용중인 닉네임입니다.");
        }
        if (response.status === 400 && errorText.includes("nickname is required")) {
          throw new Error("닉네임을 입력해주세요.");
        }
        throw new Error(`게스트 세션 생성에 실패했습니다. (${response.status})`);
      }

      const data = (await response.json()) as SessionSnapshot;
      applySession(data);
      appendEvent("GUEST_OK");
      setNotice({ kind: "info", message: "게스트 세션이 생성되었습니다." });
    } catch (error) {
      const message = error instanceof Error ? error.message : "unknown session error";
      setLastError(message);
      appendEvent(`GUEST_ERROR ${message}`);
    }
  }, [applySession, appendEvent, clearLastError, nickname, setLastError]);

  const signUpWithSupabase = useCallback(async () => {
    clearLastError();
    setNotice(null);
    try {
      requireSupabaseConfig();
      if (!email.trim() || !password.trim()) {
        throw new Error("email and password are required");
      }
      if (!nickname.trim()) {
        throw new Error("닉네임을 입력해주세요.");
      }

      const nicknameCheckResponse = await fetch("/v1/auth/nickname/check", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ nickname })
      });
      if (!nicknameCheckResponse.ok) {
        throw new Error("닉네임 중복 확인에 실패했습니다. 잠시 후 다시 시도해주세요.");
      }
      const nicknameCheck = (await nicknameCheckResponse.json()) as NicknameCheckResponse;
      if (!nicknameCheck.available) {
        throw new Error("이미 사용중인 닉네임입니다.");
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
      const rawPayload = await response.text();
      const payload = parseJSONSafely<SupabaseSignupResponse>(rawPayload);

      if (!response.ok) {
        const detail =
          payload?.msg ?? payload?.error_description ?? payload?.error ?? rawPayload.trim();
        if (detail.toLowerCase().includes("already")) {
          throw new Error("이미 가입된 이메일입니다. 로그인해주세요.");
        }
        throw new Error(`회원가입에 실패했습니다. (${response.status}) ${detail}`);
      }

      const signupDetail =
        payload?.msg ?? payload?.error_description ?? payload?.error ?? "";
      if (signupDetail.toLowerCase().includes("already")) {
        throw new Error("이미 가입된 이메일입니다. 로그인해주세요.");
      }
      const identities = payload?.user?.identities;
      if (Array.isArray(identities) && identities.length === 0) {
        throw new Error("이미 가입된 이메일입니다. 로그인해주세요.");
      }

      appendEvent("SIGNUP_OK verify your email");
      setNotice({ kind: "success", message: "회원가입이 완료되었습니다. 이메일 인증 링크를 확인해주세요." });
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
    setNotice(null);
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
      setNotice({ kind: "success", message: "로그인 및 세션 교환이 완료되었습니다." });
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
    selfSeatedRef.current = false;
  }, []);

  useEffect(() => {
    return () => {
      closeSocket();
    };
  }, [closeSocket]);

  const logout = useCallback(async () => {
    clearLastError();
    closeSocket();
    setStatus("idle");
    setSnapshot(null);
    setNotice(null);
    try {
      await fetch("/v1/auth/logout", {
        method: "POST",
        credentials: "include"
      });
    } finally {
      clearSession();
      void mutate("/v1/session/current", null, { revalidate: false });
      void mutate("/v1/wallet", undefined, { revalidate: false });
      appendEvent("LOGOUT");
    }
  }, [appendEvent, clearLastError, clearSession, closeSocket, mutate]);

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
          const nextSnapshot = parsed.payload as TableSnapshot;
          const selfPlayerID = session?.player_id ?? "";
          if (selfPlayerID !== "") {
            const wasSeated = selfSeatedRef.current;
            const isSeatedNow = nextSnapshot.players.some((player) => player.player_id === selfPlayerID);
            selfSeatedRef.current = isSeatedNow;
            if (wasSeated && !isSeatedNow) {
              void mutate("/v1/wallet");
            }
          }
          setSnapshot(nextSnapshot);
        }
        if (parsed.event_type === "error_notice") {
          const noticePayload = parsed.payload as { code?: string; message?: string };
          const message = noticePayload.message ?? "unknown websocket error";
          setLastError(message);
          appendEvent(`WS_NOTICE ${noticePayload.code ?? "UNKNOWN"} ${message}`);
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
  }, [appendEvent, clearLastError, closeSocket, mutate, session, setLastError]);

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
        const errorText = (await response.text()).trim().toLowerCase();
        if (response.status === 409 && errorText.includes("insufficient wallet balance")) {
          throw new Error("잔액이 부족합니다.");
        }
        if (response.status === 400 && errorText.includes("buy-in amount")) {
          throw new Error("해당 룸의 바이인 허용 금액 범위를 확인해주세요.");
        }
        if (response.status === 404 && errorText.includes("table not found")) {
          throw new Error("존재하지 않는 테이블입니다.");
        }
        throw new Error(`바이인 요청에 실패했습니다. (${response.status})`);
      }

      const buyInResult = (await response.json()) as BuyInResponse;
      const optimisticWallet: WalletResponse = {
        player_id: buyInResult.player_id,
        user_type: session?.user_type ?? "guest",
        balance: buyInResult.balance_after,
        timestamp: new Date().toISOString()
      };
      void mutate("/v1/wallet", optimisticWallet, { revalidate: true });
      appendEvent(`BUY_IN ${selectedTable} ${Math.floor(parsedAmount)}`);
    } catch (error) {
      const message = error instanceof Error ? error.message : "unknown buy-in error";
      setLastError(message);
      appendEvent(`BUY_IN_ERROR ${message}`);
    }
  }, [appendEvent, buyInAmount, clearLastError, mutate, selectedTable, session, setLastError]);

  const seatGrid = useMemo(() => {
    const occupied = new Map<number, SnapshotPlayer>();
    snapshot?.players.forEach((player) => occupied.set(player.seat_index, player));

    return Array.from({ length: 9 }).map((_, index) => ({
      seat: index + 1,
      player: occupied.get(index) ?? null
    }));
  }, [snapshot]);

  return {
    nickname,
    setNickname,
    email,
    setEmail,
    password,
    setPassword,
    selectedTable,
    setSelectedTable,
    buyInAmount,
    setBuyInAmount,
    status,
    health,
    wallet,
    walletError: walletError instanceof Error ? walletError.message : "",
    walletLoading,
    session,
    notice,
    lastError,
    events,
    seatGrid,
    createGuestSession,
    signUpWithSupabase,
    signInWithSupabase,
    logout,
    connectSocket,
    requestBuyIn,
    joinTable,
    leaveTable
  };
}

async function fetchJSON<T>(url: string): Promise<T> {
  const response = await fetch(url, { credentials: "include" });
  if (!response.ok) {
    const errorText = (await response.text()).trim();
    throw new Error(errorText || `request failed: ${response.status}`);
  }
  return (await response.json()) as T;
}

async function fetchCurrentSession(url: string): Promise<SessionSnapshot | null> {
  const response = await fetch(url, { credentials: "include" });
  if (response.status === 401) {
    return null;
  }
  if (!response.ok) {
    const errorText = (await response.text()).trim();
    throw new Error(errorText || `request failed: ${response.status}`);
  }
  return (await response.json()) as SessionSnapshot;
}

function createRequestId() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return `req_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
}

function parseJSONSafely<T>(rawText: string): T | null {
  if (!rawText.trim()) {
    return null;
  }
  try {
    return JSON.parse(rawText) as T;
  } catch {
    return null;
  }
}
