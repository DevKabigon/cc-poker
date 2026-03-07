export type RoomInfo = {
  id: string;
  label: string;
  smallBlind: number;
  bigBlind: number;
  minBuyIn: number;
  maxBuyIn: number;
  maxPlayers: number;
  tableCount: number;
};

export type TableInfo = {
  id: string;
  roomId: string;
  index: number;
  label: string;
};

export const ROOM_CATALOG: RoomInfo[] = [
  {
    id: "room_1_2",
    label: "$1/$2",
    smallBlind: 1,
    bigBlind: 2,
    minBuyIn: 100,
    maxBuyIn: 400,
    maxPlayers: 9,
    tableCount: 10
  },
  {
    id: "room_2_5",
    label: "$2/$5",
    smallBlind: 2,
    bigBlind: 5,
    minBuyIn: 250,
    maxBuyIn: 1000,
    maxPlayers: 9,
    tableCount: 10
  },
  {
    id: "room_5_10",
    label: "$5/$10",
    smallBlind: 5,
    bigBlind: 10,
    minBuyIn: 500,
    maxBuyIn: 2000,
    maxPlayers: 9,
    tableCount: 10
  }
];

export function findRoomById(roomID: string): RoomInfo | null {
  return ROOM_CATALOG.find((room) => room.id === roomID) ?? null;
}

export function buildTablesForRoom(roomID: string): TableInfo[] {
  const room = findRoomById(roomID);
  if (!room) {
    return [];
  }

  return Array.from({ length: room.tableCount }).map((_, index) => {
    const tableIndex = index + 1;
    return {
      id: `${room.id}_table_${tableIndex}`,
      roomId: room.id,
      index: tableIndex,
      label: `${room.label} Table ${tableIndex}`
    };
  });
}

export function parseRoomIDFromTableID(tableID: string): string | null {
  const matched = /^((room_\d+_\d+)_table_\d+)$/.exec(tableID);
  if (!matched) {
    return null;
  }

  return matched[1].replace(/_table_\d+$/, "");
}
