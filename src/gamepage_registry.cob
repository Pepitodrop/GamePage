>>SOURCE FORMAT FREE
IDENTIFICATION DIVISION.
PROGRAM-ID. GAMEPAGE-REGISTRY-BUILDER.

ENVIRONMENT DIVISION.
INPUT-OUTPUT SECTION.
FILE-CONTROL.
    SELECT ROUTE-FILE ASSIGN TO "runtime/routes.tsv"
        ORGANIZATION IS LINE SEQUENTIAL.
    SELECT POLICY-FILE ASSIGN TO "runtime/policy.tsv"
        ORGANIZATION IS LINE SEQUENTIAL.
    SELECT ARCHITECTURE-FILE ASSIGN TO "runtime/architecture.json"
        ORGANIZATION IS LINE SEQUENTIAL.
    SELECT STATUS-CLIENT-FILE ASSIGN TO "assets/status.js"
        ORGANIZATION IS LINE SEQUENTIAL.

DATA DIVISION.
FILE SECTION.
FD ROUTE-FILE.
01 ROUTE-LINE PIC X(4096).
FD POLICY-FILE.
01 POLICY-LINE PIC X(256).
FD ARCHITECTURE-FILE.
01 ARCHITECTURE-LINE PIC X(1024).
FD STATUS-CLIENT-FILE.
01 STATUS-CLIENT-LINE PIC X(4096).

WORKING-STORAGE SECTION.
COPY "game_registry.cpy".
01 REGISTRY-INDEX PIC 9(2) VALUE 0.
01 OUTPUT-BUFFER PIC X(4096).
01 OUTPUT-POINTER PIC 9(4) COMP-5 VALUE 1.
01 DISPLAY-NUMBER PIC Z(8)9.

PROCEDURE DIVISION.
MAIN.
    OPEN OUTPUT ROUTE-FILE POLICY-FILE ARCHITECTURE-FILE STATUS-CLIENT-FILE
    PERFORM WRITE-ROUTES
    PERFORM WRITE-POLICY
    PERFORM WRITE-ARCHITECTURE
    PERFORM WRITE-STATUS-CLIENT
    CLOSE ROUTE-FILE POLICY-FILE ARCHITECTURE-FILE STATUS-CLIENT-FILE
    DISPLAY "Canonical COBOL registry and policy artifacts generated."
    STOP RUN.

WRITE-ROUTES.
    WRITE ROUTE-LINE FROM
        "# key|name|prefix|upstream-env|default-upstream|health-path|enabled-env|required-env|max-latency-env|default-enabled|default-required|default-max-latency-ms"
    PERFORM VARYING REGISTRY-INDEX FROM 1 BY 1
        UNTIL REGISTRY-INDEX > REGISTRY-GAME-COUNT
        MOVE REG-GAME-DEFAULT-MAX-LATENCY(REGISTRY-INDEX)
            TO DISPLAY-NUMBER
        MOVE SPACES TO OUTPUT-BUFFER
        MOVE 1 TO OUTPUT-POINTER
        STRING
            FUNCTION TRIM(REG-GAME-KEY(REGISTRY-INDEX)) DELIMITED BY SIZE
            "|" DELIMITED BY SIZE
            FUNCTION TRIM(REG-GAME-NAME(REGISTRY-INDEX)) DELIMITED BY SIZE
            "|" DELIMITED BY SIZE
            FUNCTION TRIM(REG-GAME-PREFIX(REGISTRY-INDEX)) DELIMITED BY SIZE
            "|" DELIMITED BY SIZE
            FUNCTION TRIM(REG-GAME-UPSTREAM-ENV(REGISTRY-INDEX)) DELIMITED BY SIZE
            "|" DELIMITED BY SIZE
            FUNCTION TRIM(REG-GAME-DEFAULT-UPSTREAM(REGISTRY-INDEX)) DELIMITED BY SIZE
            "|" DELIMITED BY SIZE
            FUNCTION TRIM(REG-GAME-HEALTH-PATH(REGISTRY-INDEX)) DELIMITED BY SIZE
            "|" DELIMITED BY SIZE
            FUNCTION TRIM(REG-GAME-ENABLED-ENV(REGISTRY-INDEX)) DELIMITED BY SIZE
            "|" DELIMITED BY SIZE
            FUNCTION TRIM(REG-GAME-REQUIRED-ENV(REGISTRY-INDEX)) DELIMITED BY SIZE
            "|" DELIMITED BY SIZE
            FUNCTION TRIM(REG-GAME-LATENCY-ENV(REGISTRY-INDEX)) DELIMITED BY SIZE
            "|" DELIMITED BY SIZE
            REG-GAME-DEFAULT-ENABLED(REGISTRY-INDEX) DELIMITED BY SIZE
            "|" DELIMITED BY SIZE
            REG-GAME-DEFAULT-REQUIRED(REGISTRY-INDEX) DELIMITED BY SIZE
            "|" DELIMITED BY SIZE
            FUNCTION TRIM(DISPLAY-NUMBER) DELIMITED BY SIZE
            INTO OUTPUT-BUFFER WITH POINTER OUTPUT-POINTER
        END-STRING
        WRITE ROUTE-LINE FROM OUTPUT-BUFFER
    END-PERFORM.

WRITE-POLICY.
    WRITE POLICY-LINE FROM "# COBOL-owned runtime policy"
    MOVE SPACES TO OUTPUT-BUFFER
    MOVE 1 TO OUTPUT-POINTER
    STRING
        "STATUS_CACHE_TTL|" DELIMITED BY SIZE
        FUNCTION TRIM(REGISTRY-DEFAULT-STATUS-CACHE-TTL) DELIMITED BY SIZE
        INTO OUTPUT-BUFFER WITH POINTER OUTPUT-POINTER
    END-STRING
    WRITE POLICY-LINE FROM OUTPUT-BUFFER
    MOVE REGISTRY-GAME-COUNT TO DISPLAY-NUMBER
    MOVE SPACES TO OUTPUT-BUFFER
    MOVE 1 TO OUTPUT-POINTER
    STRING
        "MAX_GAMES|" DELIMITED BY SIZE
        FUNCTION TRIM(DISPLAY-NUMBER) DELIMITED BY SIZE
        INTO OUTPUT-BUFFER WITH POINTER OUTPUT-POINTER
    END-STRING
    WRITE POLICY-LINE FROM OUTPUT-BUFFER
    MOVE REGISTRY-DEFAULT-MINIMUM-LAUNCHABLE TO DISPLAY-NUMBER
    MOVE SPACES TO OUTPUT-BUFFER
    MOVE 1 TO OUTPUT-POINTER
    STRING
        "MINIMUM_LAUNCHABLE_GAMES|" DELIMITED BY SIZE
        FUNCTION TRIM(DISPLAY-NUMBER) DELIMITED BY SIZE
        INTO OUTPUT-BUFFER WITH POINTER OUTPUT-POINTER
    END-STRING
    WRITE POLICY-LINE FROM OUTPUT-BUFFER
    WRITE POLICY-LINE FROM "OPTIONAL_FAILURES_ALLOW_READY|true"
    WRITE POLICY-LINE FROM "SLOW_GAMES_REMAIN_LAUNCHABLE|true".

WRITE-ARCHITECTURE.
    WRITE ARCHITECTURE-LINE FROM "{"
    WRITE ARCHITECTURE-LINE FROM
        '  "applicationOwner": "COBOL",'
    WRITE ARCHITECTURE-LINE FROM
        '  "canonicalRegistry": "src/game_registry.cpy",'
    WRITE ARCHITECTURE-LINE FROM
        '  "decisionEngine": "GnuCOBOL",'
    WRITE ARCHITECTURE-LINE FROM
        '  "transportLayer": "Go",'
    WRITE ARCHITECTURE-LINE FROM
        '  "policyFeatures": ['
    WRITE ARCHITECTURE-LINE FROM
        '    "enabled flags", "required flags", "latency thresholds",'
    WRITE ARCHITECTURE-LINE FROM
        '    "optional degradation", "minimum launchable games",'
    WRITE ARCHITECTURE-LINE FROM
        '    "maintenance mode", "fail-closed launch decisions"'
    WRITE ARCHITECTURE-LINE FROM "  ],"
    WRITE ARCHITECTURE-LINE FROM
        '  "generatedConsumers": ["launcher", "route registry", "runtime policy", "status client"],'
    WRITE ARCHITECTURE-LINE FROM
        '  "goResponsibilities": ["HTTP transport", "reverse proxy", "WebSockets", "service probes", "OS signals"]'
    WRITE ARCHITECTURE-LINE FROM "}".

WRITE-STATUS-CLIENT.
    WRITE STATUS-CLIENT-LINE FROM '"use strict";'
    WRITE STATUS-CLIENT-LINE FROM 'const statusNodes = new Map(Array.from(document.querySelectorAll("[data-status]")).map((node) => [node.dataset.status, node]));'
    WRITE STATUS-CLIENT-LINE FROM 'function setStatus(key, game, mode) {'
    WRITE STATUS-CLIENT-LINE FROM '  const node = statusNodes.get(key); if (!node) return;'
    WRITE STATUS-CLIENT-LINE FROM '  const launch = node.closest(".game-card")?.querySelector(".launch");'
    WRITE STATUS-CLIENT-LINE FROM '  const launchable = game?.launchable === true; const state = game?.policyState || "down";'
    WRITE STATUS-CLIENT-LINE FROM '  node.classList.remove("checking", "up", "down", "slow", "disabled");'
    WRITE STATUS-CLIENT-LINE FROM '  node.classList.add(state === "healthy" ? "up" : state === "slow" ? "slow" : state === "disabled" ? "disabled" : "down");'
    WRITE STATUS-CLIENT-LINE FROM '  const label = node.querySelector("[data-status-text]"); let text = "OFFLINE";'
    WRITE STATUS-CLIENT-LINE FROM '  if (mode === "maintenance") text = "MAINTENANCE";'
    WRITE STATUS-CLIENT-LINE FROM '  else if (game?.reason === "decision-engine-unavailable") text = "CORE ERROR";'
    WRITE STATUS-CLIENT-LINE FROM '  else if (state === "disabled") text = "DISABLED";'
    WRITE STATUS-CLIENT-LINE FROM '  else if (state === "slow") text = `SLOW${Number.isFinite(game.latencyMs) ? ` / ${game.latencyMs}MS` : ""}`;'
    WRITE STATUS-CLIENT-LINE FROM '  else if (launchable) text = `ONLINE${Number.isFinite(game.latencyMs) ? ` / ${game.latencyMs}MS` : ""}`;'
    WRITE STATUS-CLIENT-LINE FROM '  if (label) label.textContent = text;'
    WRITE STATUS-CLIENT-LINE FROM '  if (!launch) return; if (!launch.dataset.target) launch.dataset.target = launch.getAttribute("href") || "";'
    WRITE STATUS-CLIENT-LINE FROM '  if (launchable) { launch.setAttribute("href", launch.dataset.target); launch.removeAttribute("aria-disabled"); } else { launch.removeAttribute("href"); launch.setAttribute("aria-disabled", "true"); }'
    WRITE STATUS-CLIENT-LINE FROM '}'
    WRITE STATUS-CLIENT-LINE FROM 'async function refreshStatuses() {'
    WRITE STATUS-CLIENT-LINE FROM '  try {'
    WRITE STATUS-CLIENT-LINE FROM '    const response = await fetch("/api/status", {cache: "no-store", headers: {Accept: "application/json"}});'
    WRITE STATUS-CLIENT-LINE FROM '    if (!response.ok) throw new Error("status request failed"); const payload = await response.json();'
    WRITE STATUS-CLIENT-LINE FROM '    for (const [key, game] of Object.entries(payload.games || {})) setStatus(key, game, payload.mode);'
    WRITE STATUS-CLIENT-LINE FROM '  } catch (_) { for (const key of statusNodes.keys()) setStatus(key, {launchable:false, policyState:"down", reason:"decision-engine-unavailable"}, "fail-safe"); }'
    WRITE STATUS-CLIENT-LINE FROM '}'
    WRITE STATUS-CLIENT-LINE FROM 'refreshStatuses(); setInterval(refreshStatuses, 30000);'.
