# Stage 1: Build
FROM fedora AS build

# Install necessary packages
RUN yum update -y && yum install -y wget tar curl gpg

# Set working directory
WORKDIR /opt

# Copy the GPG key for OpenSearch
COPY opensearch.pgp /opt

# Set environment variables
ENV OPENSEARCH_VERSION=2.14.0
ENV OPENSEARCH=opensearch-${OPENSEARCH_VERSION}-linux-x64

# Download and verify OpenSearch
RUN set -eux ; \
    curl --retry 8 -S -L -O https://artifacts.opensearch.org/releases/bundle/opensearch/${OPENSEARCH_VERSION}/${OPENSEARCH}.tar.gz ; \
    curl --retry 8 -S -L -O https://artifacts.opensearch.org/releases/bundle/opensearch/${OPENSEARCH_VERSION}/${OPENSEARCH}.tar.gz.sig ; \
    gpg --import /opt/opensearch.pgp ; \
    gpg --verify ${OPENSEARCH}.tar.gz.sig ${OPENSEARCH}.tar.gz ; \
    mkdir -p /usr/share/opensearch ; \
    tar zxf ${OPENSEARCH}.tar.gz -C /usr/share/opensearch --strip-components=1 ; \
    rm ${OPENSEARCH}.tar.gz.sig ${OPENSEARCH}.tar.gz opensearch.pgp

# Copy entrypoint script and set permissions
COPY entrypoint.sh /usr/local/bin
RUN chmod 0775 /usr/local/bin/*.sh && \
    chmod 0700 /usr/share/opensearch/config && \
    chmod 0600 /usr/share/opensearch/config/jvm.options && \
    chmod 0600 /usr/share/opensearch/config/log4j2.properties && \
    chmod 0600 /usr/share/opensearch/config/opensearch.yml && \
    touch /usr/share/opensearch/config/unicast_hosts.txt && \
    chmod 0600 /usr/share/opensearch/config/unicast_hosts.txt && \
    rm -rf /usr/share/opensearch/logs

# Stage 2: Runtime
FROM fedora

# Install necessary packages
RUN yum update -y && yum install -y shadow-utils tini

# Create opensearch user and group
RUN groupadd --gid 1042 opensearch || true && \
    useradd --shell /bin/bash --uid 1050 --gid 1042 --create-home opensearch || true

# Copy files from build stage and set ownership
COPY --from=build --chown=opensearch:opensearch /usr/local/bin /usr/local/bin
COPY --from=build --chown=opensearch:opensearch /usr/share/opensearch /usr/share/opensearch

# Switch to opensearch user
USER opensearch

# Set working directory
WORKDIR /usr/share/opensearch

# Define entrypoint
ENTRYPOINT ["tini", "-g", "--", "/usr/local/bin/entrypoint.sh"]
