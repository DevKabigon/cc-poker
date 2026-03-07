# Commit

- 시간(JST): 2026-03-07 16:58
- 커밋 메시지: `feat: add user type schema and signup nickname precheck`

## 요약
- `users` 스키마를 확장해 `user_type`, `email`, `email_verified` 정보를 저장하도록 했다.
- 닉네임 중복 확인 API(`POST /v1/auth/nickname/check`)를 추가해 회원가입 전에 중복 여부를 검사하도록 했다.
- 게스트/인증 세션 처리 및 닉네임 점유 로직을 보강하고, 관련 테스트를 추가했다.
- 프론트 회원가입 흐름에서 닉네임 선검사와 Supabase 응답 엄격 검증을 적용해 허위 성공 메시지 케이스를 줄였다.
- 이번 작업에 대한 `docs/work-logs` 기록을 함께 반영했다.
