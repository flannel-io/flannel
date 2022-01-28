#!/bin/sh

# Copyright 2020 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Usage:
#
#   iptables-wrapper-installer.sh [--no-sanity-check]
#
# Installs a wrapper iptables script in a container that will figure out
# whether iptables-legacy or iptables-nft is in use on the host and then
# replaces itself with the correct underlying iptables version.
#
# Unless "--no-sanity-check" is passed, it will first verify that the
# container already contains a suitable version of iptables.

# NOTE: This can only use POSIX /bin/sh features; the build container
# might not contain bash.

set -eu

# Find iptables binary location
if [ -d /usr/sbin -a -e /usr/sbin/iptables ]; then
    sbin="/usr/sbin"
elif [ -d /sbin -a -e /sbin/iptables ]; then
    sbin="/sbin"
else
    echo "ERROR: iptables is not present in either /usr/sbin or /sbin" 1>&2
    exit 1
fi

# Determine how the system selects between iptables-legacy and iptables-nft
if [ -x /usr/sbin/alternatives ]; then
    # Fedora/SUSE style alternatives
    altstyle="fedora"
elif [ -x /usr/sbin/update-alternatives ]; then
    # Debian style alternatives
    altstyle="debian"
else
    # No alternatives system
    altstyle="none"
fi

if [ "${1:-}" != "--no-sanity-check" ]; then
    # Ensure dependencies are installed
    if ! version=$("${sbin}/iptables-nft" --version 2> /dev/null); then
        echo "ERROR: iptables-nft is not installed" 1>&2
        exit 1
    fi
    if ! "${sbin}/iptables-legacy" --version > /dev/null 2>&1; then
        echo "ERROR: iptables-legacy is not installed" 1>&2
        exit 1
    fi

    case "${version}" in
    *v1.8.[012]\ *)
        echo "ERROR: iptables 1.8.0 - 1.8.2 have compatibility bugs." 1>&2
        echo "       Upgrade to 1.8.3 or newer." 1>&2
        exit 1
        ;;
    *v1.8.3\ *)
	# 1.8.3 mostly works but can get stuck in an infinite loop if the nft
	# kernel modules are unavailable
	need_timeout=1
	;;
    *)
        # 1.8.4+ are OK
        ;;
    esac
fi

# Start creating the wrapper...
rm -f "${sbin}/iptables-wrapper"
cat > "${sbin}/iptables-wrapper" <<EOF
#!/bin/sh

# Copyright 2020 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# NOTE: This can only use POSIX /bin/sh features; the container image
# might not contain bash.

set -eu

# Detect whether the base system is using iptables-legacy or
# iptables-nft. This assumes that some non-containerized process (eg
# kubelet) has already created some iptables rules.
EOF

if [ "${need_timeout:-0}" = 0 ]; then
    # Write out the simpler version of legacy-vs-nft detection
    cat >> "${sbin}/iptables-wrapper" <<EOF
num_legacy_lines=\$( (iptables-legacy-save || true; ip6tables-legacy-save || true) 2>/dev/null | grep '^-' | wc -l)
num_nft_lines=\$( (iptables-nft-save || true; ip6tables-nft-save || true) 2>/dev/null | grep '^-' | wc -l)
if [ "\${num_legacy_lines}" -ge "\${num_nft_lines}" ]; then
    mode=legacy
else
    mode=nft
fi
EOF
else
    # Write out the version of legacy-vs-nft detection with an nft timeout
    cat >> "${sbin}/iptables-wrapper" <<EOF
# The iptables-nft binary in this image can get stuck in an infinite
# loop if nft is not available so we need to wrap a timeout around it
# (and to avoid that, we don't even bother calling iptables-nft if it
# looks like iptables-legacy is going to win).
num_legacy_lines=\$( (iptables-legacy-save || true; ip6tables-legacy-save || true) 2>/dev/null | grep '^-' | wc -l)
if [ "\${num_legacy_lines}" -ge 10 ]; then
    mode=legacy
else
    num_nft_lines=\$( (timeout 5 sh -c "iptables-nft-save; ip6tables-nft-save" || true) 2>/dev/null | grep '^-' | wc -l)
    if [ "\${num_legacy_lines}" -ge "\${num_nft_lines}" ]; then
        mode=legacy
    else
        mode=nft
    fi
fi
EOF
fi

# Write out the appropriate alternatives-selection commands
case "${altstyle}" in
    fedora)
cat >> "${sbin}/iptables-wrapper" <<EOF
# Update links to point to the selected binaries
alternatives --set iptables "/usr/sbin/iptables-\${mode}" > /dev/null || failed=1
EOF
    ;;

    debian)
cat >> "${sbin}/iptables-wrapper" <<EOF
# Update links to point to the selected binaries
update-alternatives --set iptables "/usr/sbin/iptables-\${mode}" > /dev/null || failed=1
update-alternatives --set ip6tables "/usr/sbin/ip6tables-\${mode}" > /dev/null || failed=1
EOF
    ;;

    *)
cat >> "${sbin}/iptables-wrapper" <<EOF
# Update links to point to the selected binaries
for cmd in iptables iptables-save iptables-restore ip6tables ip6tables-save ip6tables-restore; do
    rm -f "${sbin}/\${cmd}"
    ln -s "${sbin}/xtables-\${mode}-multi" "${sbin}/\${cmd}"
done 2>/dev/null || failed=1
EOF
    ;;
esac

# Write out the post-alternatives-selection error checking and final wrap-up
cat >> "${sbin}/iptables-wrapper" <<EOF
if [ "\${failed:-0}" = 1 ]; then
    echo "Unable to redirect iptables binaries. (Are you running in an unprivileged pod?)" 1>&2
    # fake it, though this will probably also fail if they aren't root
    exec "${sbin}/xtables-\${mode}-multi" "\$0" "\$@"
fi

# Now re-exec the original command with the newly-selected alternative
exec "\$0" "\$@"
EOF
chmod +x "${sbin}/iptables-wrapper"

# Now back in the installer script, point the iptables binaries at our
# wrapper
case "${altstyle}" in
    fedora)
	alternatives \
            --install /usr/sbin/iptables iptables /usr/sbin/iptables-wrapper 100 \
            --slave /usr/sbin/iptables-restore iptables-restore /usr/sbin/iptables-wrapper \
            --slave /usr/sbin/iptables-save iptables-save /usr/sbin/iptables-wrapper \
            --slave /usr/sbin/ip6tables iptables /usr/sbin/iptables-wrapper \
            --slave /usr/sbin/ip6tables-restore iptables-restore /usr/sbin/iptables-wrapper \
            --slave /usr/sbin/ip6tables-save iptables-save /usr/sbin/iptables-wrapper
	;;

    debian)
	update-alternatives \
            --install /usr/sbin/iptables iptables /usr/sbin/iptables-wrapper 100 \
            --slave /usr/sbin/iptables-restore iptables-restore /usr/sbin/iptables-wrapper \
            --slave /usr/sbin/iptables-save iptables-save /usr/sbin/iptables-wrapper
	update-alternatives \
            --install /usr/sbin/ip6tables ip6tables /usr/sbin/iptables-wrapper 100 \
            --slave /usr/sbin/ip6tables-restore ip6tables-restore /usr/sbin/iptables-wrapper \
            --slave /usr/sbin/ip6tables-save ip6tables-save /usr/sbin/iptables-wrapper
	;;

    *)
	for cmd in iptables iptables-save iptables-restore ip6tables ip6tables-save ip6tables-restore; do
            rm -f "${sbin}/${cmd}"
            ln -s "${sbin}/iptables-wrapper" "${sbin}/${cmd}"
	done
	;;
esac

# Cleanup
rm -f "$0"
