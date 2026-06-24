import re

with open("docker-compose.prod.yml", "r") as f:
    content = f.read()

# Revert postgres ports
content = re.sub(r'    expose:\n      - "5432"\n    ports:\n      - "127\.0\.0\.1:5432:5432"', r'    expose:\n      - "5432"', content)

# Revert ejabberd ports
content = re.sub(r'    ports:\n      - "5222:5222"\n      - "127\.0\.0\.1:5222:5222"', r'    ports:\n      - "5222:5222"', content)

# Revert livekit network_mode
content = re.sub(r'    restart: unless-stopped\n    network_mode: "host"\n    command: --redis-host 127\.0\.0\.1:6379', r'    restart: unless-stopped\n    ports:\n      - "7880:7880"\n      - "7881:7881"\n      - "7882:7882/udp"\n      - "7882:7882/tcp"\n    command: --redis-host redis:6379', content)

# Revert livekit env
content = re.sub(r'      LIVEKIT_REDIS_ADDRESS: "127\.0\.0\.1:6379"', r'      LIVEKIT_REDIS_ADDRESS: "redis:6379"', content)

# Revert backend network_mode and extra_hosts, add ports back
content = re.sub(r'      - 8\.8\.8\.8\n      - 8\.8\.4\.4\n    network_mode: "host"\n    extra_hosts:\n      - "postgres:127\.0\.0\.1"\n      - "redis:127\.0\.0\.1"\n      - "ejabberd:127\.0\.0\.1"\n      - "livekit:127\.0\.0\.1"', r'      - 8.8.8.8\n      - 8.8.4.4\n    ports:\n      - "127.0.0.1:8080:8080"\n      - "127.0.0.1:8443:8443"\n    expose:\n      - "8080"', content)

with open("docker-compose.prod.yml", "w") as f:
    f.write(content)
