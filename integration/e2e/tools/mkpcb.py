# Writes the e2e design's board, matching main.kicad_sch and power.kicad_sch.
import sys, uuid
import pcbnew
fpdir, out = sys.argv[1], sys.argv[2]
NS = uuid.UUID("2f0b6c1e-5a43-4d0e-9a52-6e2e2e2e0001")
def U(name): return str(uuid.uuid5(NS, name))
mm = pcbnew.FromMM
def V(x, y): return pcbnew.VECTOR2I(mm(x), mm(y))

b = pcbnew.BOARD()
nets = {}
for n in ["+3V3", "GND", "Net-(D1-A)"]:
    nets[n] = pcbnew.NETINFO_ITEM(b, n)
    b.Add(nets[n])

def place(lib, name, ref, value, x, y, path, padnets, rot=0):
    fp = pcbnew.FootprintLoad(f"{fpdir}/{lib}.pretty", name)
    fp.SetFPID(pcbnew.LIB_ID(lib, name))
    fp.SetReference(ref)
    fp.SetValue(value)
    fp.SetPosition(V(x, y))
    fp.SetOrientationDegrees(rot)
    fp.SetPath(pcbnew.KIID_PATH(path))
    b.Add(fp)
    for p in fp.Pads():
        p.SetNet(nets[padnets[p.GetNumber()]])
    return fp

# Footprint paths: the sheet path below the root, then the symbol's UUID.
j1 = place("Connector_PinHeader_2.54mm", "PinHeader_1x02_P2.54mm_Vertical", "J1", "Power", 105, 100,
           f"/{U('J1')}", {"1": "+3V3", "2": "GND"})
r1 = place("Resistor_SMD", "R_0603_1608Metric", "R1", "1k", 115, 100,
           f"/{U('sheet-power')}/{U('R1')}", {"1": "+3V3", "2": "Net-(D1-A)"})
d1 = place("LED_SMD", "LED_0603_1608Metric", "D1", "LED", 115, 102.54,
           f"/{U('sheet-power')}/{U('D1')}", {"1": "GND", "2": "Net-(D1-A)"})

def pad(fp, n):
    return next(p for p in fp.Pads() if p.GetNumber() == n).GetPosition()

def track(a, c, net):
    t = pcbnew.PCB_TRACK(b)
    t.SetStart(a)
    t.SetEnd(c)
    t.SetWidth(mm(0.25))
    t.SetLayer(pcbnew.F_Cu)
    t.SetNet(nets[net])
    b.Add(t)

track(pad(j1, "1"), pad(r1, "1"), "+3V3")
track(pad(j1, "2"), pad(d1, "1"), "GND")
track(pad(r1, "2"), pad(d1, "2"), "Net-(D1-A)")

def edge(x1, y1, x2, y2):
    s = pcbnew.PCB_SHAPE(b)
    s.SetShape(pcbnew.SHAPE_T_SEGMENT)
    s.SetStart(V(x1, y1))
    s.SetEnd(V(x2, y2))
    s.SetLayer(pcbnew.Edge_Cuts)
    s.SetWidth(mm(0.1))
    b.Add(s)

for seg in [(100, 95, 122, 95), (122, 95, 122, 107), (122, 107, 100, 107), (100, 107, 100, 95)]:
    edge(*seg)

pcbnew.SaveBoard(out, b)
for fp in (j1, r1, d1):
    print(fp.GetReference(), [(p.GetNumber(), pcbnew.ToMM(p.GetPosition().x), pcbnew.ToMM(p.GetPosition().y), p.GetNetname()) for p in fp.Pads()])
