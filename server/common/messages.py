from datetime import datetime
from .utils import Bet as StoreBet

class MessageType:
    BET = 0
    RESULT = 1

class Bet:
    def __init__(self, name, surname, document, birthdate, number, msg_id, agency):
        self.name = name
        self.surname = surname
        self.document = document
        self.birthdate = birthdate
        self.number = number
        self.msg_id = msg_id
        self.agency = agency
    
    def into_store_bet(self) -> StoreBet:
        return StoreBet(str(self.agency), self.name, self.surname, self.document, self.birthdate, self.number)


class Result:
    def __init__(self, msg_id, success):
        self.msg_id = msg_id
        self.success = success

def read_message(conn) -> Bet:
    msg_type = read_bytes(conn, 1)[0]
    if msg_type == MessageType.BET:
        return read_bet(conn)
    else:
        raise ValueError(f"Unknown message type: {msg_type}")

def read_bet(conn) -> Bet:
    name_len = int.from_bytes(read_bytes(conn, 2), byteorder='big')
    name = read_bytes(conn, name_len).decode('utf-8')
    surname_len = int.from_bytes(read_bytes(conn, 2), byteorder='big')
    surname = read_bytes(conn, surname_len).decode('utf-8')
    document = read_bytes(conn, 8).decode('utf-8')
    birthdate  = read_bytes(conn, 10).decode('utf-8')
    number = read_bytes(conn, 4).decode('utf-8')
    msg_id = read_bytes(conn, 1)[0]
    agency = read_bytes(conn, 1)[0]
    
    return Bet(
        name,
        surname,
        document,
        birthdate,
        number,
        msg_id,
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


