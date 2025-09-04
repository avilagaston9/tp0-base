import socket
import logging
import signal
import threading
from .messages import read_message, Result, write_result, ReadError, MsgTypeError, WriteError, Bet, Batch, Finished, Winners, NotReady
from .utils import store_bets, load_bets, has_won
SOCKET_TIMEOUT = 0.5  # seconds

bets_lock = threading.Lock()

class Server:
    def __init__(self, port, listen_backlog, agency_count):
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)
        self._server_socket.settimeout(SOCKET_TIMEOUT)
        self._finished_agencies: set[int] = set()
        self.graceful_shutdown = False
        self._agency_count = agency_count
        signal.signal(signal.SIGTERM, self.__handle_sigterm)

    def __handle_sigterm(self, signum, frame):  
        logging.info("action: graceful_shutdown | result: in_progress")
        self.graceful_shutdown = True
    
    def run(self):
        while not self.graceful_shutdown:
            client_sock = self.__try_accept_new_connection()
            if client_sock:
                threading.Thread(
                    target=self.__handle_client_connection, 
                    args=(client_sock,),
                    daemon=True
                ).start()
        self._server_socket.close()
        logging.info("action: close_main_socket | result: success ")
        logging.info("action: graceful_shutdown | result: success")

    def __try_accept_new_connection(self):
        """
        Accept new connections
        Function blocks until a connection to a client is made.
        Then connection created is printed and returned.
        If the configured timeout is reached, returns None
        """
        try:
            # Connection arrived
            logging.info('action: accept_connections | result: in_progress')
            c, addr = self._server_socket.accept()
            logging.info(f'action: accept_connections | result: success | ip: {addr[0]}')
            return c
        except socket.timeout:
            return None
        except Exception as e:
            logging.error(f"action: accept_connection | result: fail | error: {e}")
        return None

    def __handle_client_connection(self, client_sock):
        """
        Read message from a specific client socket and closes the socket
        If a problem arises in the communication with the client, the
        client socket will also be closed
        """
        try:
            client_sock.settimeout(SOCKET_TIMEOUT)
            addr = client_sock.getpeername()
            msg_id, msg = read_message(client_sock)
            self.handle_message(msg_id, msg, client_sock)
        except WriteError as e:
            logging.error(f"action: send_message | result: fail | error: {e}]")
        except ReadError as e:
            logging.error(f"action: receive_message | result: fail | error: {e}]")
        except MsgTypeError as e:
            logging.error(f"action: receive_message | result: fail | error: {e}]")
        except Exception as e:
            logging.error(f"action: handle_connection | result: fail | error: {e}")
        finally:
            client_sock.close()
            logging.info(f"action: close_client_socket | result: success | ip: {addr[0]}")

    def handle_message(self, msg_id, msg, client_sock) -> Result:
        if isinstance(msg, Bet):
            # Process single bet
            success =  process_bet(msg)
            write_result(client_sock, Result(msg_id, success))
        elif isinstance(msg, Batch):
            # Process batch of bets
            success = process_batch(msg)
            write_result(client_sock, Result(msg_id, success))
        elif isinstance(msg, Finished):
            logging.info(f"action: agencia_finalizada | result: success | agency: {msg.agency}")
            self._process_finished_agency(client_sock, msg.agency)
        else:
            logging.error(f"action: apuesta_almacenada | result: fail | error: Unexpected message type {type(msg)}")

    # TODO: Send msgId
    def _process_finished_agency(self, client_sock, agency):
        # Aquire lock to update finished agencies set
        with bets_lock:
            self._finished_agencies.add(agency)
            finished_agencies_count = len(self._finished_agencies)

        if finished_agencies_count == self._agency_count:
            logging.info("action: sorteo | result: success")
            winner_documents = get_winner_documents(agency)
            client_sock.sendall(winner_documents.to_bytes())
        else:
            logging.info(f"action: sorteo | result: in_progress | agencias_finalizadas: {len(self._finished_agencies)}")
            client_sock.sendall(NotReady().to_bytes())



def process_bet(bet) -> bool:
    with bets_lock:
        store_bets([bet.into_store_bet()])
    logging.info(f'action: apuesta_almacenada | result: success | dni: {bet.document} | numero: {bet.number}')
    return True

def process_batch(batch) -> bool:
    with bets_lock:
        store_bets(batch.bets)
    logging.info(f"action: apuesta_recibida | result: success | cantidad: {len(batch.bets)}")
    return True
    
def get_winner_documents(agency) -> Winners:
    with bets_lock:
        bets = load_bets()
    winners_documents = [int(bet.document) for bet in bets if has_won(bet) and bet.agency == agency]
    return Winners(winners_documents)
