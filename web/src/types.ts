export const PROTOCOL_VERSION = "1";

// ---- Client → Server messages ----

export interface MoveMessage {
  type: "move";
  v?: string;
  from: string;
  to: string;
  promotion?: string;
}

export interface NewGameMessage {
  type: "new_game";
  v?: string;
  aiMode: string;
  aiDepth: number;
}

export interface ResignMessage {
  type: "resign";
  v?: string;
}

export interface CreateRoomMessage {
  type: "create_room";
  v?: string;
  aiMode: string;
  aiDepth: number;
  color: string; // "white" | "black" | "random"
  timeControl?: string; // preset name or "none"
}

export interface DrawOfferMessage {
  type: "draw_offer";
  v?: string;
}

export interface DrawResponseMessage {
  type: "draw_response";
  v?: string;
  accept: boolean;
}

export interface JoinRoomMessage {
  type: "join_room";
  v?: string;
  roomId: string;
}

export interface ListRoomsMessage {
  type: "list_rooms";
  v?: string;
}

export interface RegisterMessage {
  type: "register";
  v?: string;
  username: string;
  password: string;
}

export interface LoginMessage {
  type: "login";
  v?: string;
  username: string;
  password: string;
}

// ---- Server → Client messages ----

export interface LegalMove {
  from: string;
  to: string;
  promotion?: string;
}

export interface GameState {
  type: "state";
  v: string;
  board: string[][];
  turn: "white" | "black";
  moveNumber: number;
  isCheck: boolean;
  isGameOver: boolean;
  result: string;
  legalMoves: LegalMove[];
  lastMove: LegalMove | null;
  moveHistory: string[];
  playerColor?: "white" | "black" | "spectator";
  roomId?: string;
  // Clock (ms remaining per side; 0/absent when untimed)
  whiteMs?: number;
  blackMs?: number;
  hasClock?: boolean;
  // Draw state
  drawOffered?: boolean;
  claimableDraw?: string;
}

export interface ErrorMsg {
  type: "error";
  v: string;
  message: string;
}

export interface ThinkingMsg {
  type: "thinking";
  v: string;
}

export interface SessionMessage {
  type: "session";
  v: string;
  playerId: string;
  displayName: string;
  token: string;
}

export interface RoomCreatedMessage {
  type: "room_created";
  v: string;
  roomId: string;
  playerColor: string;
}

export interface RoomJoinedMessage {
  type: "room_joined";
  v: string;
  roomId: string;
  playerColor: string;
  opponentName: string;
}

export interface RoomListMessage {
  type: "room_list";
  v: string;
  rooms: RoomInfo[];
}

export interface OpponentDisconnectedMessage {
  type: "opponent_disconnected";
  v: string;
}

export interface OpponentReconnectedMessage {
  type: "opponent_reconnected";
  v: string;
}

export interface DrawOfferedServerMessage {
  type: "draw_offered";
  v: string;
}

export interface RatingUpdateMessage {
  type: "rating_update";
  v: string;
  oldRating: number;
  newRating: number;
  delta: number;
}

export interface RoomInfo {
  roomId: string;
  status: "waiting" | "playing" | "finished";
  whiteName: string;
  blackName: string;
  whiteRating?: number;
  blackRating?: number;
  timeControl?: string;
  spectators?: number;
}

export interface AuthOKMessage {
  type: "auth_ok";
  v: string;
  token: string;
  playerId: string;
  displayName: string;
}

export interface FindGameMessage {
  type: "find_game";
  v?: string;
  timeControl?: string;
}

export interface CancelMatchMessage {
  type: "cancel_match";
  v?: string;
}

export interface UndoMessage {
  type: "undo";
  v?: string;
}

export type ClientMessage =
  | MoveMessage
  | NewGameMessage
  | ResignMessage
  | CreateRoomMessage
  | JoinRoomMessage
  | ListRoomsMessage
  | DrawOfferMessage
  | DrawResponseMessage
  | RegisterMessage
  | LoginMessage
  | FindGameMessage
  | CancelMatchMessage
  | UndoMessage;

export interface MatchFoundMessage {
  type: "match_found";
  v: string;
  roomId: string;
  playerColor: string;
  opponentName: string;
}

export interface MatchWaitingMessage {
  type: "match_waiting";
  v: string;
  timeControl: string;
}

export interface MatchCancelledMessage {
  type: "match_cancelled";
  v: string;
}

export type ServerMessage =
  | GameState
  | ErrorMsg
  | ThinkingMsg
  | SessionMessage
  | RoomCreatedMessage
  | RoomJoinedMessage
  | RoomListMessage
  | OpponentDisconnectedMessage
  | OpponentReconnectedMessage
  | DrawOfferedServerMessage
  | RatingUpdateMessage
  | AuthOKMessage
  | MatchFoundMessage
  | MatchWaitingMessage
  | MatchCancelledMessage;
