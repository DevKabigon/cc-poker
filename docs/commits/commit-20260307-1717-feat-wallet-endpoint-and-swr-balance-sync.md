# Commit

- 시간(JST): 2026-03-07 17:17
- 커밋 메시지: `feat: add wallet endpoint and swr balance sync`

## 요약
- 백엔드에 `GET /v1/wallet`을 추가하고 `EventStore`에 지갑 잔액 조회 인터페이스를 확장했다.
- Postgres/noop 저장소 모두 `GetWalletBalance`를 구현하고, 지갑 조회 테스트를 추가했다.
- 프론트 `usePlayConsole`에 SWR 지갑 캐시를 연결하고 바이인 성공 시 `balance_after` 기반 `mutate`로 즉시 반영되도록 했다.
- Auth/Play 상태 카드에 지갑 잔액 표시를 추가하고 세션 타입에 `user_type` 필드를 확장했다.
- 작업 로그(`docs/work-logs/work-20260307-1714-23.md`)를 추가했다.
