# Phase 0 - 이벤트 스키마 초안 (v0.1)

## 목적
- 서버(Go)와 클라이언트(React/Phaser)가 동일한 실시간 프로토콜을 공유한다.
- 이후 `pkg/protocol` 구현 시 기준으로 사용한다.

## 0. 인증/연결 전제
- WebSocket 핸드셰이크 인증은 `HttpOnly 세션 쿠키` 기반으로 처리한다.
- 서버는 업그레이드 요청 시 세션 쿠키 검증 후에만 연결을 수락한다.
- 교차 출처 요청은 `Origin` 검증으로 제한한다.

## 1. 공통 이벤트 Envelope
```json
{
  "event_version": 1,
  "event_type": "player_action",
  "table_id": "tbl_123",
  "hand_id": "h_456",
  "seq": 1024,
  "sent_at": "2026-03-07T03:00:00Z",
  "payload": {}
}
```

## 필드 정의
- `event_version`: 프로토콜 버전
- `event_type`: 이벤트 이름
- `table_id`: 테이블 식별자
- `hand_id`: 핸드 식별자 (로비/입장 이벤트는 nullable 가능)
- `seq`: 테이블 단위 단조 증가 시퀀스
- `sent_at`: 서버 기준 UTC 시간
- `payload`: 이벤트 본문

## 2. 클라이언트 -> 서버 이벤트

## 2.1 `join_table`
```json
{
  "event_type": "join_table",
  "payload": {
    "table_id": "tbl_123",
    "seat_index": 3
  }
}
```

## 2.2 `leave_table`
```json
{
  "event_type": "leave_table",
  "payload": {
    "table_id": "tbl_123"
  }
}
```

## 2.3 `player_action`
```json
{
  "event_type": "player_action",
  "payload": {
    "action_id": "act_789",
    "action_type": "raise",
    "amount": 120
  }
}
```

## `action_type` 허용값
- `fold`
- `check`
- `call`
- `bet`
- `raise`
- `all_in`

## 2.4 `request_state_sync`
```json
{
  "event_type": "request_state_sync",
  "payload": {
    "last_seq": 1010
  }
}
```

## 3. 서버 -> 클라이언트 이벤트

## 3.1 `table_snapshot`
재접속/입장 직후 전체 상태 전달용
```json
{
  "event_type": "table_snapshot",
  "payload": {
    "table_state": "playing",
    "max_players": 9,
    "min_players_to_start": 2,
    "button_seat": 4,
    "sb_seat": 5,
    "bb_seat": 6,
    "board": ["As", "Kd", "7h"],
    "pot_total": 560,
    "players": []
  }
}
```

## 3.2 `hand_started`
```json
{
  "event_type": "hand_started",
  "payload": {
    "hand_id": "h_456",
    "button_seat": 2,
    "sb_seat": 3,
    "bb_seat": 4
  }
}
```

## 3.3 `turn_started`
```json
{
  "event_type": "turn_started",
  "payload": {
    "player_id": "u_001",
    "round": "preflop",
    "to_call": 20,
    "min_raise_to": 40,
    "base_action_ms": 30000,
    "timebank_remaining_ms": 60000
  }
}
```

## 3.4 `action_applied`
```json
{
  "event_type": "action_applied",
  "payload": {
    "player_id": "u_001",
    "action_type": "call",
    "amount": 20,
    "pot_total": 60,
    "next_player_id": "u_002"
  }
}
```

## 3.5 `round_changed`
```json
{
  "event_type": "round_changed",
  "payload": {
    "round": "flop",
    "board": ["As", "Kd", "7h"]
  }
}
```

## 3.6 `hand_result`
```json
{
  "event_type": "hand_result",
  "payload": {
    "hand_id": "h_456",
    "winners": [
      {
        "player_id": "u_003",
        "amount": 1240,
        "hand_rank": "two_pair"
      }
    ],
    "showdown": true
  }
}
```

## 3.7 `error_notice`
```json
{
  "event_type": "error_notice",
  "payload": {
    "code": "INVALID_ACTION",
    "message": "invalid action for current turn",
    "request_action_id": "act_789"
  }
}
```

## 4. 상태 동기화 규칙
- 클라이언트는 `seq` 기반으로 이벤트를 적용한다.
- 중간 시퀀스 누락 감지 시 `request_state_sync`를 보낸다.
- 서버는 필요 시 `table_snapshot`으로 상태를 재기준화한다.

## 5. 멱등성/중복 처리 규칙
- `action_id`는 클라이언트가 UUID로 생성한다.
- 서버는 최근 `action_id`를 캐시해 중복 액션을 무시한다.
- 동일 `action_id` 재수신 시 동일 결과를 반환해야 한다.

## 6. 확정 필요 항목 (TODO)
1. `table_snapshot.players[]` 상세 필드 확정
2. `error_notice.code` 표준 목록 확정
3. 관전자 전용 이벤트 분리 여부 결정
4. 이벤트 압축/배치 전송 전략 필요 여부 검토
