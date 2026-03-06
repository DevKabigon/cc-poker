# Phase 0 - 아키텍처 초안 (v0.1)

## 1. 아키텍처 목표
- 서버 권위형으로 승패/칩 정합성 보장
- 2~9인 테이블 단위 동시성 제어 단순화
- 재접속과 장애 대응 가능한 상태 동기화 구조 확보

## 2. 시스템 구성

## 2.1 클라이언트 (React + Phaser)
- React:
  - 로비/테이블 UI/입력 패널
  - 네트워크 상태(연결, 복구, 오류) 표시
- Phaser:
  - 카드/칩 애니메이션 렌더링
  - 게임 룰 계산 없음(표현 전용)

## 2.2 API/실시간 게이트웨이 (Go)
- REST:
  - 인증, 로비 조회, 프로필
- WebSocket:
  - 테이블 입장/액션/브로드캐스트
  - seq 기반 이벤트 흐름
  - 인증 방식: `HttpOnly 세션 쿠키` 기반 핸드셰이크
  - 보안: `Origin` 검증 + `SameSite` 정책 적용

## 2.3 게임 도메인 (Go)
- Table Runtime:
  - 테이블 단위 이벤트 루프(직렬 처리)
  - 턴 진행/타이머/상태 전환
- Poker Engine:
  - 핸드 판정, 사이드팟 정산
  - 액션 유효성 검증

## 2.4 데이터 계층
- PostgreSQL:
  - 계정, 칩 지갑, 핸드 히스토리
- Redis:
  - 세션, 레이트 리밋, 단기 캐시
  - 테이블 스냅샷 저장/복구(재기동 대비)

## 3. 런타임 흐름
1. 사용자가 로비에서 테이블 선택
2. 클라이언트가 `join_table` 이벤트 전송
3. 서버가 좌석 배정 후 `table_snapshot` 반환
4. 핸드 시작 시 `hand_started` -> `turn_started` 순서 브로드캐스트
5. 플레이어 액션마다 서버 검증 후 `action_applied` 브로드캐스트
6. 핸드 종료 시 `hand_result` 전송 및 정산 저장

## 4. 동기화/복구 전략
- 이벤트마다 `seq`를 부여해 순서 보장
- 누락 감지 시 `request_state_sync`로 스냅샷 재수신
- 연결 재수립 시:
  - 최신 `table_snapshot`
  - 필요 시 최근 이벤트 재생

## 5. 스케일링 전략 (MVP -> 확장)
- MVP:
  - 단일 게임 서버 인스턴스 + 수직 확장 우선
- 확장:
  - 테이블 ID 기반 샤딩
  - 게임 서버 다중 인스턴스 + sticky/session routing
  - 중앙 Pub/Sub(필요 시 Redis Stream/NATS 검토)

## 6. 장애/예외 처리
- WS 끊김:
  - 플레이어 상태 `disconnected`
  - 타임아웃 정책으로 자동 액션 진행
- 서버 재시작:
  - 이벤트 로그 + 스냅샷 기반 복구
- DB 장애:
  - 신규 테이블 생성 차단
  - 진행 중 테이블은 메모리 기반 임시 지속

## 7. 보안 경계
- 신뢰 경계:
  - 클라이언트 입력은 절대 신뢰하지 않음
- 검증 계층:
  - 인증/인가 -> 스키마 검증 -> 도메인 검증 순 적용
- 감사 로깅:
  - 거절된 액션, 비정상 요청, 과도한 재시도 기록

## 8. 초기 디렉터리 제안
```txt
cc-poker/
  backend/
    cmd/server/
    internal/app/
    internal/game/
    internal/table/
    internal/ws/
    internal/store/
    pkg/protocol/
  web/
    src/features/lobby/
    src/features/table-ui/
    src/features/table-scene/
    src/shared/net/
    src/shared/state/
```

## 9. Phase 1 이전 확정 항목
1. `table_snapshot` 상세 필드 확정
2. 이벤트 저장 전략 (전체 이벤트 vs 핵심 이벤트)
3. 단일 리포 vs 멀티 리포 최종 결정
