# The Andrew File System
The Andrew File System: https://pages.cs.wisc.edu/~remzi/OSTEP/dist-afs.pdf

# Andrew File System Overview

The Andrew File System implements a straightforward client-server architecture for distributed file management. The system consists of a central server and multiple client nodes.

### Architecture
The distributed file synchronization system consists of two main components: **a server** and **multiple clients**. The server acts as a centralized hub, managing file storage and synchronization while maintaining authoritative copies of all files. It handles incoming client requests for file operations and implements a callback mechanism to ensure proper synchronization across the system.

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
    S-->>S: Update master/server copy
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

### Client Components
On the client side, multiple nodes can access and modify files through both automatic and manual means. Each client implements folder monitoring for automatic change detection and provides a CLI interface for manual file operations. Clients are responsible for maintaining their local file state and ensuring proper synchronization with the server.

![Client Components](docs/images/client-components.png)

### Core Operations
The system supports two key file management operations: `FETCH`, `STORE` and `DELETE`. The `STORE` operation handles both creating new files and updating existing ones, while `DELETE` removes files from the system. These operations can be triggered either through the automatic folder monitoring system or via manual CLI commands.

```mermaid
flowchart TD
    Start([Start Operation])

    Start --> FileOp{Operation Type?}
    FileOp -->|STORE| CheckExists{File Exists?}
    
    
    CheckExists -->|No| NewFile[Create New File]
    NewFile --> Broadcast1[Broadcast to Clients]
    
    %% Update Flow
    CheckExists -->|Yes| CompareChecksum{Compare Checksums}
    CompareChecksum -->|Different| UpdateFile[Update File Content]
    UpdateFile --> Broadcast2[Broadcast to Clients]
    CompareChecksum -->|Same| NoAction[No Action Needed]
    
    
    FileOp -->|DELETE| CheckDeletion{File Exists?}
    CheckDeletion -->|Yes| AddTombstone[Add to Tombstone]
    AddTombstone --> Broadcast3[Broadcast Deletion]
    CheckDeletion -->|No| NoActionNeeded[No Action Needed]
    
    
    classDef process fill:#68a063,stroke:#333,stroke-width:2px,color:white
    classDef decision fill:#c3963a,stroke:#333,stroke-width:2px,color:white
    classDef endpoint fill:#5a7bb5,stroke:#333,stroke-width:2px,color:white
    
    class Start,NoAction,NoActionNeeded endpoint
    class CheckExists,CompareChecksum,CheckDeletion decision
    class NewFile,UpdateFile,AddTombstone,Broadcast1,Broadcast2,Broadcast3 process
```

### Synchronization Architecture
The synchronization mechanism employs a "train station" model for callbacks using gRPC. Clients initiate continuous synchronization requests to the server, and the system uses file checksums and modification timestamps to track changes. Clients maintain active connections to receive updates whenever changes occur in the system.


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

### AFS Server Command

```
Usage:
  ./afs-server [flags]

Flags:
  -h, --help           help for afs-server
      --mount string   Storage directory for files (default "./mount/server")
      --port int       The server port (default 50051)
```


### AFS Client Command

```
Usage:
  ./afs-client [command]

Available Commands:
  delete      Delete a file from the system
  fetch       Fetch a file from the system
  help        Help about any command
  store       Store a file in the system
  watch       Watch the mount directory for changes and sync with AFS

Flags:
  -h, --help            help for afs-client
      --id string       client identifier
      --mount string    Storage directory for client cache (default "./tmp/client")
      --server string   server address (default "localhost:50051")
```