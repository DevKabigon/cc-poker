import { Navigate, Route, Routes } from "react-router-dom";
import { PlayPage } from "../pages/play/PlayPage";
import { PolicyPage } from "../pages/policy/PolicyPage";
import { AuthPage } from "../pages/auth/AuthPage";

export function AppRouter() {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/auth" replace />} />
      <Route path="/auth" element={<AuthPage />} />
      <Route path="/play" element={<PlayPage />} />
      <Route path="/policy" element={<PolicyPage />} />
      <Route path="*" element={<Navigate to="/auth" replace />} />
    </Routes>
  );
}
