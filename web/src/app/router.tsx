import { Navigate, Route, Routes } from "react-router-dom";
import { PolicyPage } from "../pages/policy/PolicyPage";
import { AuthPage } from "../pages/auth/AuthPage";
import { LobbyPage } from "../pages/lobby/LobbyPage";
import { RoomPage } from "../pages/rooms/RoomPage";
import { TablePage } from "../pages/tables/TablePage";

export function AppRouter() {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/auth" replace />} />
      <Route path="/auth" element={<AuthPage />} />
      <Route path="/lobby" element={<LobbyPage />} />
      <Route path="/rooms/:roomId" element={<RoomPage />} />
      <Route path="/tables/:tableId" element={<TablePage />} />
      <Route path="/play" element={<Navigate to="/lobby" replace />} />
      <Route path="/policy" element={<PolicyPage />} />
      <Route path="*" element={<Navigate to="/auth" replace />} />
    </Routes>
  );
}
