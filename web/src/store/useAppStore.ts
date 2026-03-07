import { create } from "zustand";

export type SessionSnapshot = {
  player_id: string;
  nickname: string;
  user_type?: string;
  expires_at: string;
  email?: string;
  email_verified?: boolean;
};

type AppStore = {
  session: SessionSnapshot | null;
  lastError: string;
  setSession: (session: SessionSnapshot | null) => void;
  clearSession: () => void;
  setLastError: (error: string) => void;
  clearLastError: () => void;
};

export const useAppStore = create<AppStore>((set) => ({
  session: null,
  lastError: "",
  setSession: (session) => set({ session }),
  clearSession: () => set({ session: null }),
  setLastError: (error) => set({ lastError: error }),
  clearLastError: () => set({ lastError: "" })
}));
