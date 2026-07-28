FROM odoo:18.0-20260723

COPY addons/ /mnt/extra-addons/

USER root

COPY requirements.txt /tmp/requirements.txt
RUN PIP_BREAK_SYSTEM_PACKAGES=1 python3 -m pip install --no-cache-dir -r /tmp/requirements.txt

COPY packages.txt /tmp/packages.txt
RUN DEBIAN_FRONTEND=noninteractive apt-get update \
    && xargs -a /tmp/packages.txt apt-get install -y --no-install-recommends \
    && rm -rf /var/lib/apt/lists/*

USER odoo
