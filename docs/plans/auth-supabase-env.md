# Supabase Auth 연동 환경변수

## Backend
- `CC_POKER_SUPABASE_ENABLED=true`
- `CC_POKER_SUPABASE_URL=https://YOUR_PROJECT_REF.supabase.co`
- `CC_POKER_SUPABASE_ANON_KEY=YOUR_SUPABASE_ANON_KEY`
- `CC_POKER_SUPABASE_TIMEOUT_MS=1500`
- `CC_POKER_GUEST_WALLET_INITIAL=2000`
- `CC_POKER_AUTH_WALLET_INITIAL=10000`

## Frontend (Vite)
- `VITE_SUPABASE_URL=https://YOUR_PROJECT_REF.supabase.co`
- `VITE_SUPABASE_ANON_KEY=YOUR_SUPABASE_ANON_KEY`

## 정책
- 게스트: 최초 세션 생성 시 서버 지갑 2,000 지급
- 로그인 + 이메일 인증 유저: 최초 지갑 생성 시 10,000 지급
- 실제 잔액은 서버 DB 지갑 기준으로만 관리

