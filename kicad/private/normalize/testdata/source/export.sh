#!/bin/bash
# export.sh TREE BOARD_DIR OUT: runs every kicad-cli export the normalizer covers.
T=$1; B=$2; O=$3; mkdir -p $O/gerbers $O/drill; H=$(mktemp -d); cd $B
k() { timeout 120 env -i HOME=$H TMPDIR=$H PATH=/nonexistent LC_ALL=C.UTF-8 XDG_CONFIG_HOME=$H/c XDG_CACHE_HOME=$H/x $T/bin/kicad-cli "$@" >> $O/log.txt 2>&1 || echo "FAILED: $*" >> $O/log.txt; }
k pcb export gerbers -o $O/gerbers/ fixture.kicad_pcb
k pcb export drill -o $O/drill/ --generate-map --map-format gerberx2 fixture.kicad_pcb
k pcb export pdf -o $O/board.pdf --layers F.Cu,Edge.Cuts fixture.kicad_pcb
k pcb export step -o $O/board.step fixture.kicad_pcb
k pcb drc --format json -o $O/drc.json fixture.kicad_pcb
k sch erc --format json -o $O/erc.json fixture.kicad_sch
k sch export pdf -o $O/sch.pdf fixture.kicad_sch
rm -rf $H
