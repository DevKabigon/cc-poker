# Phase 0 - 개발 컨벤션 초안 (Go 중심)

## 목적
- TypeScript 개발 경험을 Go로 자연스럽게 이전할 수 있도록 기준을 통일한다.
- 유지보수성을 위해 주석/네이밍/에러 처리 규칙을 먼저 고정한다.

## 1. 언어별 역할
- Go: 게임 룰 엔진, 실시간 서버, 데이터 정합성 책임
- TypeScript/React/Phaser: UI/연출/상태 표현 책임
- 원칙: 승패/칩/덱 계산은 Go 서버에서만 수행한다.

## 2. Go 주석 규칙 (한국어)
- 공개 식별자(타입/함수/메서드/상수)에는 한국어 문서 주석을 작성한다.
- "무엇"보다 "왜"를 설명한다.
- 도메인 용어는 한국어 + 영문 병기 허용:
  - 예: `사이드팟(side pot)`, `스몰 블라인드(SB)`
- 복잡한 로직(포지션 로테이션, 사이드팟 정산)에는 블록 주석 1~3줄 추가
- 불필요한 주석은 금지:
  - 코드만 읽어도 명확하면 주석 생략

## 3. Go 코드 스타일
- 패키지는 도메인 단위로 분리:
  - `internal/game`, `internal/table`, `internal/ws`, `internal/store`
- 함수는 한 가지 책임만 가지도록 작게 유지
- 에러는 래핑해서 맥락 전달:
  - `fmt.Errorf("failed to apply action: %w", err)`
- `panic` 사용 금지(프로세스 시작 실패 같은 초기화 단계 제외)
- 동시성은 테이블 단위 직렬 처리(Actor-like loop) 우선

## 4. TypeScript -> Go 전환 가이드
- TS의 `union` 기반 액션 타입은 Go에서 `enum + struct` 조합으로 표현
- TS 런타임 검증(zod 등)은 Go에서 명시적 validator 함수로 대체
- 불변 상태 업데이트 감각은 Go에서도 유지:
  - 핸드 단위 변경은 명확한 함수 경계에서만 수행

## 5. 예시 코드
```go
package game

import "fmt"

// ApplyAction은 현재 턴의 플레이어 액션을 검증하고
// 게임 상태에 반영한 뒤, 다음 턴 정보를 반환한다.
func ApplyAction(state *TableState, action PlayerAction) (*TurnResult, error) {
	// 현재 턴 플레이어가 아니면 즉시 거절한다.
	if action.PlayerID != state.CurrentTurnPlayerID {
		return nil, fmt.Errorf("invalid turn: player=%s", action.PlayerID)
	}

	// 허용 액션/금액 범위를 검증한다.
	if err := validateAction(state, action); err != nil {
		return nil, fmt.Errorf("invalid action: %w", err)
	}

	// 상태 반영과 다음 턴 계산은 반드시 동일 트랜잭션 경계에서 처리한다.
	result, err := applyAndAdvance(state, action)
	if err != nil {
		return nil, fmt.Errorf("failed to apply action: %w", err)
	}

	return result, nil
}
```

## 6. PR 체크리스트 (Go 코드용)
1. 공개 함수/타입에 한국어 주석이 있는가?
2. 승패/칩 로직이 서버에서만 실행되는가?
3. 타임아웃/재접속/중복 액션 케이스를 테스트했는가?
4. 에러 메시지에 디버깅 가능한 맥락이 포함되는가?
5. 컨트롤러/핸들러가 게임 도메인 로직을 직접 소유하지 않는가?
