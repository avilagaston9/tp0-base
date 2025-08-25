from datetime import datetime
from .utils import Bet as StoreBet

class Message:
    """Base class for all message types."""
    pass

class MessageType:
    BET = 0
    RESULT = 1
    BATCH = 2

class Batch(Message):
    def __init__(self, bets):
        self.bets = bets
    

class Bet(Message):
    def __init__(self, name, surname, document, birthdate, number, agency):
        self.name = name
        self.surname = surname
        self.document = document
        self.birthdate = birthdate
        self.number = number
        self.agency = agency
    
    def into_store_bet(self) -> StoreBet:
        return StoreBet(str(self.agency), self.name, self.surname, str(self.document), self.birthdate, str(self.number))


class Result(Message):
    def __init__(self, msg_id, success):
        self.msg_id = msg_id
        self.success = success

def read_message(conn):
    msg_id = read_bytes(conn, 1)[0]
    msg_type = read_bytes(conn, 1)[0]
    if msg_type == MessageType.BET:
        return msg_id, read_bet(conn)
    elif msg_type == MessageType.BATCH:
        return msg_id, read_batch(conn)
    else:
        raise ValueError(f"Unknown message type: {msg_type}")

def read_batch(conn) -> Batch:
    num_bets = int.from_bytes(read_bytes(conn, 2), byteorder='big')
    bets = []
    for _ in range(num_bets):
        bet = read_bet(conn)
        bets.append(bet.into_store_bet())
    return Batch(bets)


def read_bet(conn) -> Bet:
    name_len = int.from_bytes(read_bytes(conn, 1), byteorder='big')
    name = read_bytes(conn, name_len).decode('utf-8')
    surname_len = int.from_bytes(read_bytes(conn, 1), byteorder='big')
    surname = read_bytes(conn, surname_len).decode('utf-8')
    document = int.from_bytes(read_bytes(conn, 4), byteorder='big')
    birthdate  = read_bytes(conn, 10).decode('utf-8')
    number = int.from_bytes(read_bytes(conn, 4), byteorder='big')
    agency = int.from_bytes(read_bytes(conn, 1), byteorder='big')
    
    return Bet(
        name,
        surname,
        document,
        birthdate,
        number,
        agency
    )

def read_bytes(conn, num_bytes: int) -> bytes:
    buffer = bytearray()
    while len(buffer) < num_bytes:
        chunk = conn.recv(num_bytes - len(buffer))
        if not chunk:
            raise ConnectionError("Connection closed")
        buffer.extend(chunk)
    return bytes(buffer)


def write_result(conn, result: Result):
    message_bytes = (
        bytes([MessageType.RESULT]) +
        bytes([result.msg_id]) +
        bytes([int(result.success)])
    )
    conn.sendall(message_bytes)


