# Commit

- 시간(JST): 2026-03-08 00:46
- 커밋 메시지: `refactor: split app package and add poker table ui`

## 요약
- `backend/internal/app/app.go`를 동작 변경 없이 역할별 파일로 분할해 유지보수성을 높였다.
- HTTP/WS 핸들러, 세션/상태 보조, 에러 매핑, auth 보조, JSON 응답 유틸을 각각 분리했다.
- 테이블 페이지에 실제 포커 테이블 형태 UI(`PokerTableView`)를 도입했다.
- 테이블 보드/좌석/카드 스타일을 추가해 기존 단순 좌석 그리드를 대체했다.
- 이번 작업에 대한 work-log(`work-20260308-0040-30.md`)를 반영했다.
