import sys
import pcbnew

out = sys.argv[1]
mm = pcbnew.FromMM
b = pcbnew.BOARD()

def line(x1, y1, x2, y2, layer, w=0.1):
    s = pcbnew.PCB_SHAPE(b)
    s.SetShape(pcbnew.SHAPE_T_SEGMENT)
    s.SetStart(pcbnew.VECTOR2I(mm(x1), mm(y1)))
    s.SetEnd(pcbnew.VECTOR2I(mm(x2), mm(y2)))
    s.SetLayer(layer)
    s.SetWidth(mm(w))
    b.Add(s)

for x1, y1, x2, y2 in [(0, 0, 20, 0), (20, 0, 20, 12), (20, 12, 0, 12), (0, 12, 0, 0)]:
    line(x1, y1, x2, y2, pcbnew.Edge_Cuts)

net = pcbnew.NETINFO_ITEM(b, "SIG")
b.Add(net)

fp = pcbnew.FOOTPRINT(b)
fp.SetFPID(pcbnew.LIB_ID("Fixture", "Two_Pad"))
fp.SetReference("J1")
fp.SetValue("Fixture")
fp.SetPosition(pcbnew.VECTOR2I(mm(10), mm(6)))
b.Add(fp)

def pad(num, x, y, tht):
    p = pcbnew.PAD(fp)
    p.SetNumber(num)
    p.SetPosition(pcbnew.VECTOR2I(mm(x), mm(y)))
    if tht:
        p.SetAttribute(pcbnew.PAD_ATTRIB_PTH)
        p.SetShape(pcbnew.PAD_SHAPE_CIRCLE)
        p.SetSize(pcbnew.VECTOR2I(mm(1.7), mm(1.7)))
        p.SetDrillSize(pcbnew.VECTOR2I(mm(1.0), mm(1.0)))
        p.SetLayerSet(pcbnew.PAD.PTHMask())
    else:
        p.SetAttribute(pcbnew.PAD_ATTRIB_SMD)
        p.SetShape(pcbnew.PAD_SHAPE_RECT)
        p.SetSize(pcbnew.VECTOR2I(mm(1.0), mm(1.2)))
        p.SetLayerSet(pcbnew.PAD.SMDMask())
    fp.Add(p)
    return p

p1 = pad("1", 5, 6, True)
pad("2", 15, 6, True)
pad("3", 8, 3, False)
pad("4", 12, 3, False)
p1.SetNet(net)

t = pcbnew.PCB_TRACK(b)
t.SetStart(pcbnew.VECTOR2I(mm(5), mm(6)))
t.SetEnd(pcbnew.VECTOR2I(mm(10), mm(9)))
t.SetWidth(mm(0.25))
t.SetLayer(pcbnew.F_Cu)
t.SetNet(net)
b.Add(t)

v = pcbnew.PCB_VIA(b)
v.SetPosition(pcbnew.VECTOR2I(mm(10), mm(9)))
v.SetWidth(mm(0.6))
v.SetDrill(mm(0.3))
v.SetNet(net)
b.Add(v)

txt = pcbnew.PCB_TEXT(b)
txt.SetText("FIXTURE")
txt.SetPosition(pcbnew.VECTOR2I(mm(10), mm(10.5)))
txt.SetLayer(pcbnew.F_SilkS)
b.Add(txt)

pcbnew.SaveBoard(out, b)
print("saved", out)
