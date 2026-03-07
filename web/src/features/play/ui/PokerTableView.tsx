import { Badge } from "@/components/ui/badge";
import type { SnapshotPlayer } from "../model/types";

type SeatSlot = {
  seat: number;
  player: SnapshotPlayer | null;
};

type PokerTableViewProps = {
  seats: SeatSlot[];
};

const SEAT_POSITIONS = [
  { top: "10%", left: "50%" },
  { top: "20%", left: "80%" },
  { top: "45%", left: "92%" },
  { top: "74%", left: "83%" },
  { top: "88%", left: "50%" },
  { top: "74%", left: "17%" },
  { top: "45%", left: "8%" },
  { top: "20%", left: "20%" },
  { top: "5%", left: "32%" }
] as const;

export function PokerTableView({ seats }: PokerTableViewProps) {
  return (
    <div className="poker-table-shell">
      <div className="poker-felt">
        <div className="poker-center">
          <p className="poker-label">Community Cards</p>
          <div className="community-cards">
            {Array.from({ length: 5 }).map((_, index) => (
              <div key={`community-${index}`} className="playing-card ghost-card">
                ?
              </div>
            ))}
          </div>
          <p className="poker-label">Pot: $0</p>
        </div>

        {seats.map((slot, index) => {
          const position = SEAT_POSITIONS[index] ?? SEAT_POSITIONS[0];
          const occupied = slot.player !== null;
          return (
            <div
              key={`seat-${slot.seat}`}
              className={`seat-node ${occupied ? "occupied" : ""}`}
              style={{ top: position.top, left: position.left }}
            >
              <div className="seat-node__meta">
                <span>Seat {slot.seat}</span>
                {occupied ? <Badge variant="secondary">{slot.player?.stack}</Badge> : <Badge variant="outline">Open</Badge>}
              </div>

              <strong>{occupied ? slot.player?.nickname : "Waiting"}</strong>

              <div className="hole-cards">
                <div className={`playing-card ${occupied ? "" : "ghost-card"}`}>{occupied ? "A" : "?"}</div>
                <div className={`playing-card ${occupied ? "" : "ghost-card"}`}>{occupied ? "K" : "?"}</div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}