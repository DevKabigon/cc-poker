# Commit

- 시간(JST): 2026-03-07 16:11
- 커밋 메시지: `feat: add theme system and enforce guest nickname validation`

## 요약
- 프론트에 `next-themes` 기반 라이트/다크/시스템 테마 전환을 추가했다.
- shadcn 컴포넌트와 페이지 스타일을 테마 토큰 중심으로 재정리해 가독성과 hover 대비를 개선했다.
- Auth/Play/Policy/TopNav UI를 분리/정돈하고 네비게이션 렌더링을 데이터 기반으로 정리했다.
- 게스트 세션 생성 시 닉네임 필수 검증을 프론트/백엔드에 모두 적용했다.
- health 체크를 폴링에서 1회 체크 방식으로 변경했다.
- 관련 작업 로그를 `docs/work-logs`에 추가했다.
