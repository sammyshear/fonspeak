form Pitch
  sentence File_name
  positive New_pitch
endform
snd = Read from file: file_name$
selectObject(snd)


manipulation = To Manipulation: 0.01, 90, 3000
pitchtier = Extract pitch tier

original = Copy: "old"
points = Get number of points


for p to points
  selectObject(original)
  t = Get time from index: p
  selectObject(pitchtier)
  Remove point: p
  Add point: t, new_pitch
endfor

selectObject(pitchtier, manipulation)
Replace pitch tier

selectObject(manipulation)
new_snd = Get resynthesis (overlap-add)

removeObject(original, pitchtier, manipulation)
selectObject(new_snd)
Rename: "modified"
Save as WAV file: file_name$ + "_out.wav"
