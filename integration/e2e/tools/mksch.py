# Writes the e2e design's two sheets from KiCad 9.0.9 library symbols.
import sys, uuid
libdir, out = sys.argv[1], sys.argv[2]
NS = uuid.UUID("2f0b6c1e-5a43-4d0e-9a52-6e2e2e2e0001")
def U(name): return str(uuid.uuid5(NS, name))

def extract(lib, name):
    s = open(f"{libdir}/{lib}.kicad_sym").read()
    i = s.index(f'\n\t(symbol "{name}"\n') + 1
    depth, j = 0, i
    while True:
        c = s[j]
        if c == '"':
            j += 1
            while s[j] != '"':
                j += 2 if s[j] == '\\' else 1
        elif c == '(':
            depth += 1
        elif c == ')':
            depth -= 1
            if depth == 0:
                break
        j += 1
    body = s[i:j + 1]
    return body.replace(f'(symbol "{name}"', f'(symbol "{lib}:{name}"', 1)

def indent(text, n):
    return "\n".join(("\t" * n) + l[1:] if l.startswith("\t") else ("\t" * n) + l for l in text.splitlines())

ROOT, SHEET = U("root"), U("sheet-power")
PROJECT = "e2e"

def prop(k, v, x, y, hide=False):
    h = "\n\t\t\t\t(hide yes)" if hide else ""
    return f'''\t\t(property "{k}" "{v}"
\t\t\t(at {x} {y} 0)
\t\t\t(effects
\t\t\t\t(font
\t\t\t\t\t(size 1.27 1.27)
\t\t\t\t){h}
\t\t\t)
\t\t)'''

def place(lib_id, ref, value, fp, x, y, pins, path, in_bom=True):
    props = [
        prop("Reference", ref, x + 2.54, y - 1.27, hide=ref.startswith("#")),
        prop("Value", value, x + 2.54, y + 1.27),
        prop("Footprint", fp, x, y, hide=True),
        prop("Datasheet", "~", x, y, hide=True),
        prop("Description", "", x, y, hide=True),
    ]
    pin_s = "\n".join(f'\t\t(pin "{p}"\n\t\t\t(uuid "{U(ref + "-pin" + p)}")\n\t\t)' for p in pins)
    yn = "yes" if in_bom else "no"
    return f'''\t(symbol
\t\t(lib_id "{lib_id}")
\t\t(at {x} {y} 0)
\t\t(unit 1)
\t\t(exclude_from_sim no)
\t\t(in_bom {yn})
\t\t(on_board {yn})
\t\t(dnp no)
\t\t(uuid "{U(ref)}")
{chr(10).join(props)}
{pin_s}
\t\t(instances
\t\t\t(project "{PROJECT}"
\t\t\t\t(path "{path}"
\t\t\t\t\t(reference "{ref}")
\t\t\t\t\t(unit 1)
\t\t\t\t)
\t\t\t)
\t\t)
\t)'''

def sheetfile(uid, libs, body, extra):
    syms = "\n".join(indent(extract(l, n), 2) for l, n in libs)
    return f'''(kicad_sch
\t(version 20250114)
\t(generator "eeschema")
\t(generator_version "9.0")
\t(uuid "{uid}")
\t(paper "A4")
\t(lib_symbols
{syms}
\t)
{body}
{extra}\t(embedded_fonts no)
)
'''

main_body = "\n".join([
    place("Connector_Generic:Conn_01x02", "J1", "Power", "Connector_PinHeader_2.54mm:PinHeader_1x02_P2.54mm_Vertical", 50.8, 50.8, ["1", "2"], f"/{ROOT}"),
    place("power:+3V3", "#PWR01", "+3V3", "", 45.72, 50.8, ["1"], f"/{ROOT}", in_bom=False),
    place("power:GND", "#PWR02", "GND", "", 45.72, 53.34, ["1"], f"/{ROOT}", in_bom=False),
    place("power:PWR_FLAG", "#FLG01", "PWR_FLAG", "", 45.72, 50.8, ["1"], f"/{ROOT}", in_bom=False),
    place("power:PWR_FLAG", "#FLG02", "PWR_FLAG", "", 45.72, 53.34, ["1"], f"/{ROOT}", in_bom=False),
    f'''\t(sheet
\t\t(at 76.2 38.1)
\t\t(size 25.4 12.7)
\t\t(exclude_from_sim no)
\t\t(in_bom yes)
\t\t(on_board yes)
\t\t(dnp no)
\t\t(stroke
\t\t\t(width 0.1524)
\t\t\t(type solid)
\t\t)
\t\t(fill
\t\t\t(color 0 0 0 0.0000)
\t\t)
\t\t(uuid "{SHEET}")
\t\t(property "Sheetname" "Power"
\t\t\t(at 76.2 37.3884 0)
\t\t\t(effects
\t\t\t\t(font
\t\t\t\t\t(size 1.27 1.27)
\t\t\t\t)
\t\t\t\t(justify left bottom)
\t\t\t)
\t\t)
\t\t(property "Sheetfile" "power.kicad_sch"
\t\t\t(at 76.2 51.3846 0)
\t\t\t(effects
\t\t\t\t(font
\t\t\t\t\t(size 1.27 1.27)
\t\t\t\t)
\t\t\t\t(justify left top)
\t\t\t)
\t\t)
\t\t(instances
\t\t\t(project "{PROJECT}"
\t\t\t\t(path "/{ROOT}"
\t\t\t\t\t(page "2")
\t\t\t\t)
\t\t\t)
\t\t)
\t)''',
])
main_libs = [("Connector_Generic", "Conn_01x02"), ("power", "+3V3"), ("power", "GND"), ("power", "PWR_FLAG")]
open(f"{out}/main.kicad_sch", "w").write(sheetfile(ROOT, main_libs, main_body, '''\t(sheet_instances
\t\t(path "/"
\t\t\t(page "1")
\t\t)
\t)
'''))

sub = f"/{ROOT}/{SHEET}"
power_body = "\n".join([
    place("Device:R", "R1", "1k", "Resistor_SMD:R_0603_1608Metric", 101.6, 76.2, ["1", "2"], sub),
    place("Device:LED", "D1", "LED", "LED_SMD:LED_0603_1608Metric", 97.79, 80.01, ["1", "2"], sub),
    place("power:+3V3", "#PWR03", "+3V3", "", 101.6, 72.39, ["1"], sub, in_bom=False),
    place("power:GND", "#PWR04", "GND", "", 93.98, 80.01, ["1"], sub, in_bom=False),
])
open(f"{out}/power.kicad_sch", "w").write(sheetfile(U("power-file"), [("Device", "LED"), ("Device", "R"), ("power", "+3V3"), ("power", "GND")], power_body, ""))
print("wrote", out)
