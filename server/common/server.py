import socket
import logging
import signal

SOCKET_TIMEOUT = 0.5  # seconds

class Server:
    def __init__(self, port, listen_backlog):
        # Initialize server socket
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)
        self._server_socket.settimeout(SOCKET_TIMEOUT)
        self.graceful_shutdown = False
        signal.signal(signal.SIGTERM, self.__handle_sigterm)

    def __handle_sigterm(self, signum, frame):  
        logging.info("action: graceful_shutdown | result: in_progress")
        self.graceful_shutdown = True
    
    def run(self):
        """
        Dummy Server loop

        Server that accept a new connections and establishes a
        communication with a client. After client with communucation
        finishes, servers starts to accept new connections again
        """

        # the server
        while not self.graceful_shutdown:
            client_sock = self.__accept_new_connection()
            if client_sock:
                self.__handle_client_connection(client_sock)
        self._server_socket.close()
        logging.info("action: close_main_socket | result: success ")
        logging.info("action: graceful_shutdown | result: success")


    def __handle_client_connection(self, client_sock):
        """
        Read message from a specific client socket and closes the socket

        If a problem arises in the communication with the client, the
        client socket will also be closed
        """
        try:
            # TODO: Modify the receive to avoid short-reads
            client_sock.settimeout(SOCKET_TIMEOUT)
            msg = client_sock.recv(1024).rstrip().decode('utf-8')
            addr = client_sock.getpeername()
            logging.info(f'action: receive_message | result: success | ip: {addr[0]} | msg: {msg}')
            # TODO: Modify the send to avoid short-writes
            client_sock.send("{}\n".format(msg).encode('utf-8'))
        except socket.timeout:
            pass
        except OSError as e:
            logging.error("action: receive_message | result: fail | error: {e}")
        finally:
            client_sock.close()
            logging.info(f"action: close_client_socket | result: success | ip: {addr[0]}")

    def __accept_new_connection(self):
        """
        Accept new connections

        Function blocks until a connection to a client is made.
        Then connection created is printed and returned.
        If the timeout of 1 second is reached, returns None
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
