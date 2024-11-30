# The Andrew File System
The Andrew File System: https://pages.cs.wisc.edu/~remzi/OSTEP/dist-afs.pdf

# Andrew File System Overview

The Andrew File System implements a straightforward client-server architecture for distributed file management. The system consists of a central server and multiple client nodes.


## Architecture
```mermaid
sequenceDiagram
    participant C1 as Client 1
    participant C2 as Client 2
    participant S as Server

    Note over C1,C2: Multiple clients connected
    
    % Initial connection
    C1->>S: Initialize connection
    C2->>S: Initialize connection
    
    % File change detection
    Note over C1: File change detected<br/>(monitor/CLI)
    
    % Store operation
    C1->>S: STORE operation
    activate S
    S-->>S: Update master copy
    S-->>S: Calculate new checksum
    
    % Broadcast to other clients
    par Broadcast to clients
        S->>C1: Sync notification
        S->>C2: Sync notification
    end
    deactivate S
    
    % Client 2 sync
    activate C2
    C2->>S: Request file data
    S-->>C2: Send updated file
    C2-->>C2: Update local copy
    deactivate C2
    
```

### Architecture Overview
The distributed file synchronization system consists of two main components: `a server` and `multiple clients`. The server acts as a centralized hub, managing file storage and synchronization while maintaining authoritative copies of all files. It handles incoming client requests for file operations and implements a callback mechanism to ensure proper synchronization across the system.

### Client Components
On the client side, multiple nodes can access and modify files through both automatic and manual means. Each client implements folder monitoring for automatic change detection and provides a CLI interface for manual file operations. Clients are responsible for maintaining their local file state and ensuring proper synchronization with the server.

### Core Operations
The system supports two key file management operations: `STORE` and `DELETE`. The `STORE` operation handles both creating new files and updating existing ones, while `DELETE` removes files from the system. These operations can be triggered either through the automatic folder monitoring system or via manual CLI commands.

### Synchronization Architecture
The synchronization mechanism employs a "train station" model for callbacks using gRPC. Clients initiate continuous synchronization requests to the server, and the system uses file checksums and modification timestamps to track changes. Clients maintain active connections to receive updates whenever changes occur in the system.

### Process Flow
First, a client detects changes either through folder monitoring or CLI commands. The client then initiates communication with the server, which compares checksums and modification times to determine what has changed. Finally, the server broadcasts these changes to all connected clients through the callback mechanism, and the clients update their local files to match the server's state.

## Get Started

Setup
```sh
make setup
```

Start Server
```sh
./bin/afs-server --mount=./tmp/server
```

Client 1
```sh
./bin/afs-client --id client1 --mount ./tmp/client1 watch
```

Client 2
```sh
./bin/afs-client --id client2 --mount ./tmp/client2 watch
```

Client 3
```sh
./bin/afs-client --id client3 --mount ./tmp/client3 watch
```
