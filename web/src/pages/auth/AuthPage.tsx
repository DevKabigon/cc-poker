import { TopNav } from "../../features/navigation/ui/TopNav";
import { usePlayConsole } from "../../features/play/model/usePlayConsole";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Link } from "react-router-dom";

export function AuthPage() {
  const {
    nickname,
    setNickname,
    email,
    setEmail,
    password,
    setPassword,
    session,
    notice,
    lastError,
    health,
    createGuestSession,
    signUpWithSupabase,
    signInWithSupabase,
    logout
  } = usePlayConsole();

  return (
    <main className="page">
      <TopNav />

      <section className="hero">
        <p className="eyebrow">CC Poker Authentication</p>
        <h1>Guest / Login Session</h1>
        <p className="subtitle">
          게스트는 2,000칩, 이메일 인증 유저는 10,000칩 정책으로 세션을 생성합니다.
        </p>
      </section>

      <section className="layout">
        <Card className="panel">
          <CardHeader>
            <CardTitle>세션 생성</CardTitle>
            <CardDescription>게스트 또는 Supabase 계정으로 접속합니다.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="field" style={{ maxWidth: "100%" }}>
              <Label htmlFor="nickname">Nickname</Label>
              <Input
                id="nickname"
                value={nickname}
                onChange={(event) => setNickname(event.target.value)}
                maxLength={20}
              />
            </div>

            <div className="field" style={{ maxWidth: "100%" }}>
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                type="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="you@example.com"
              />
            </div>

            <div className="field" style={{ maxWidth: "100%" }}>
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                placeholder="min 6 chars"
              />
            </div>

            <div className="button-row" style={{ marginTop: 0 }}>
              <Button onClick={createGuestSession}>게스트 시작</Button>
              <Button variant="outline" onClick={signUpWithSupabase}>
                회원가입
              </Button>
              <Button variant="outline" onClick={signInWithSupabase}>
                로그인
              </Button>
              <Button variant="outline" onClick={logout} disabled={!session}>
                로그아웃
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

        <Card className="panel">
          <CardHeader>
            <CardTitle>현재 상태</CardTitle>
            <CardDescription>세션 생성 이후 Play Console에서 소켓/테이블을 진행하세요.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <p>
              Backend: <Badge variant="outline">{health?.status ?? "unknown"}</Badge>
            </p>
            <p>
              Session:{" "}
              <Badge variant={session ? "default" : "secondary"}>
                {session ? session.player_id : "none"}
              </Badge>
            </p>
            <p>
              Nickname: <Badge variant="outline">{session?.nickname ?? "-"}</Badge>
            </p>
            <p>
              Email Verified:{" "}
              <Badge variant={session?.email_verified ? "default" : "secondary"}>
                {session?.email_verified ? "yes" : "no"}
              </Badge>
            </p>
            <Button asChild>
              <Link to="/play">Play Console로 이동</Link>
            </Button>
          </CardContent>
        </Card>
      </section>
    </main>
  );
}
