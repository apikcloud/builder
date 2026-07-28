FROM odoo:18.0-20260723

COPY addons/ /mnt/extra-addons/

USER root

COPY packages.txt /tmp/packages.txt
RUN DEBIAN_FRONTEND=noninteractive apt-get update \
    && grep -vE '^[[:space:]]*(#|$)' /tmp/packages.txt | xargs apt-get install -y --no-install-recommends \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt /tmp/requirements.txt
RUN PIP_BREAK_SYSTEM_PACKAGES=1 python3 -m pip install --no-cache-dir -r /tmp/requirements.txt

USER odoo
