export type SnapshotPlayer = {
  player_id: string;
  nickname: string;
  seat_index: number;
  stack: number;
};

export type TableSnapshot = {
  table_state: string;
  max_players: number;
  min_players_to_start: number;
  active_players: number;
  can_start: boolean;
  players: SnapshotPlayer[];
};

export type ServerEnvelope = {
  event_type: string;
  table_id?: string;
  payload: unknown;
  sent_at: string;
};

export type SupabaseLoginResponse = {
  access_token: string;
};

export type SupabaseSignupResponse = {
  user?: {
    identities?: unknown[] | null;
  } | null;
  session?: unknown | null;
  error?: string;
  error_description?: string;
  msg?: string;
};

export type NicknameCheckResponse = {
  available: boolean;
};

export type HealthResponse = {
  status: string;
  service: string;
  timestamp: string;
};

export type ConnectionStatus = "idle" | "connecting" | "connected" | "closed" | "error";

export type UiNotice = {
  kind: "success" | "info";
  message: string;
} | null;
