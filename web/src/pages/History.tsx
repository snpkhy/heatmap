import CompareArrowsRoundedIcon from "@mui/icons-material/CompareArrowsRounded";
import DeleteOutlineRoundedIcon from "@mui/icons-material/DeleteOutlineRounded";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Checkbox from "@mui/material/Checkbox";
import Chip from "@mui/material/Chip";
import IconButton from "@mui/material/IconButton";
import Link from "@mui/material/Link";
import Paper from "@mui/material/Paper";
import Stack from "@mui/material/Stack";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableContainer from "@mui/material/TableContainer";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";
import { useCallback, useEffect, useState } from "react";
import { api, type PipelineInfo, type RunListItem, type RunStatus, runFile } from "../api";
import { dateTime, fps, metric, noiseLabel } from "../format";
import { useT } from "../i18n";
import { navigate, routeHref } from "../router";
import { fonts } from "../theme";

export const statusColor: Record<
  RunStatus,
  "default" | "primary" | "success" | "error" | "warning"
> = {
  queued: "default",
  running: "primary",
  done: "success",
  failed: "error",
  cancelled: "warning",
};

export function History() {
  const t = useT();
  const [runs, setRuns] = useState<RunListItem[] | null>(null);
  const [modes, setModes] = useState<PipelineInfo[]>([]);
  const [selected, setSelected] = useState<string[]>([]);
  const [error, setError] = useState("");

  const load = useCallback(() => {
    api
      .runs()
      .then((r) => {
        setRuns(r);
        setError("");
      })
      .catch((e: Error) => setError(e.message));
  }, []);

  useEffect(load, [load]);
  // Mode names for runs that are still in progress (finished ones carry their own).
  useEffect(() => {
    api
      .pipelines()
      .then(setModes)
      .catch(() => {});
  }, []);

  // Refresh while something is still in progress.
  const active = runs?.some((r) => r.status === "running" || r.status === "queued");
  useEffect(() => {
    if (!active) return;
    const timer = setInterval(load, 1000);
    return () => clearInterval(timer);
  }, [active, load]);

  const toggle = (id: string) =>
    setSelected((s) => (s.includes(id) ? s.filter((x) => x !== id) : [...s, id]));

  const remove = async (r: RunListItem) => {
    try {
      await api.deleteRun(r.run_id);
      setSelected((s) => s.filter((x) => x !== r.run_id));
      load();
    } catch (e) {
      setError((e as Error).message);
    }
  };

  const modeName = (r: RunListItem) => {
    const id = r.summary?.pipeline ?? r.request?.pipeline;
    return t.modeName(id, r.summary?.pipeline_name ?? modes.find((m) => m.id === id)?.name ?? "");
  };

  return (
    <Stack spacing={3}>
      <Stack
        direction="row"
        spacing={2}
        sx={{ alignItems: "flex-end", justifyContent: "space-between", flexWrap: "wrap" }}
      >
        <Box>
          <Typography variant="h2" component="h2" sx={{ fontSize: { xs: 34, md: 42 } }}>
            {t.history.title}
          </Typography>
          <Typography color="text.secondary" sx={{ mt: 0.5 }}>
            {t.history.lead}
          </Typography>
        </Box>
        <Button
          variant="contained"
          startIcon={<CompareArrowsRoundedIcon />}
          disabled={selected.length < 2}
          onClick={() => navigate({ page: "compare", ids: selected })}
        >
          {t.history.compare(selected.length)}
        </Button>
      </Stack>
      {error && <Alert severity="error">{error}</Alert>}
      {runs && runs.length === 0 && (
        <Alert severity="info">
          {t.history.empty[0]}
          <Link href={routeHref({ page: "launch" })}>{t.history.empty[1]}</Link>
          {t.history.empty[2]}
        </Alert>
      )}
      {runs && runs.length > 0 && (
        <TableContainer component={Paper} variant="outlined" sx={{ borderRadius: "16px" }}>
          <Table aria-label={t.history.table}>
            <TableHead>
              <TableRow>
                <TableCell padding="checkbox" />
                <TableCell>{t.history.scene}</TableCell>
                <TableCell>{t.history.status}</TableCell>
                <TableCell>{t.history.mode}</TableCell>
                <TableCell>{t.history.noise}</TableCell>
                <TableCell align="right">{t.history.accuracy}</TableCell>
                <TableCell align="right">{t.history.speed}</TableCell>
                <TableCell>{t.history.when}</TableCell>
                <TableCell />
              </TableRow>
            </TableHead>
            <TableBody>
              {runs.map((r) => {
                const s = r.summary;
                const done = r.status === "done";
                const progress = r.progress && r.progress.total > 0 ? r.progress : undefined;
                const scene = s?.sequence ?? r.request?.sequence ?? r.run_id;
                return (
                  <TableRow key={r.run_id} hover>
                    <TableCell padding="checkbox">
                      <Checkbox
                        size="small"
                        disabled={!done}
                        checked={selected.includes(r.run_id)}
                        onChange={() => toggle(r.run_id)}
                        slotProps={{ input: { "aria-label": t.history.selectLabel(r.run_id) } }}
                      />
                    </TableCell>
                    <TableCell>
                      <Link
                        href={routeHref({ page: "run", id: r.run_id })}
                        underline="none"
                        color="inherit"
                        aria-label={t.history.openLabel(r.run_id)}
                        sx={{ display: "flex", alignItems: "center", gap: 1.5 }}
                      >
                        <Box
                          sx={{
                            width: 72,
                            aspectRatio: "4 / 3",
                            flex: "none",
                            borderRadius: "8px",
                            overflow: "hidden",
                            bgcolor: "#07060a",
                          }}
                        >
                          {done && (
                            <Box
                              component="img"
                              src={runFile(r.run_id, "heatmap.png")}
                              alt=""
                              loading="lazy"
                              sx={{ width: "100%", height: "100%", objectFit: "cover" }}
                            />
                          )}
                        </Box>
                        <Box sx={{ minWidth: 0 }}>
                          <Typography
                            sx={{
                              fontFamily: fonts.display,
                              fontSize: 17,
                              fontWeight: 600,
                              letterSpacing: "-0.02em",
                              lineHeight: 1.2,
                            }}
                          >
                            {t.sceneTitle(scene)}
                          </Typography>
                          <Typography
                            variant="caption"
                            color="text.secondary"
                            sx={{ fontFamily: fonts.mono, fontSize: 11 }}
                          >
                            {s ? t.frames(s.frames) : scene}
                          </Typography>
                        </Box>
                      </Link>
                    </TableCell>
                    <TableCell>
                      <Tooltip title={r.error ?? ""}>
                        <Chip
                          size="small"
                          variant="outlined"
                          color={statusColor[r.status]}
                          label={
                            r.status === "running" && progress
                              ? `${progress.done}/${progress.total}`
                              : t.status[r.status]
                          }
                        />
                      </Tooltip>
                    </TableCell>
                    <TableCell>{modeName(r)}</TableCell>
                    <TableCell>{noiseLabel(s ? s.noise : r.request?.noise, t.noise)}</TableCell>
                    <TableCell align="right" sx={{ fontFamily: fonts.mono }}>
                      {metric(s?.f1)}
                    </TableCell>
                    <TableCell align="right" sx={{ fontFamily: fonts.mono }}>
                      {s ? `${fps(s.fps)} ${t.units.fps}` : "—"}
                    </TableCell>
                    <TableCell sx={{ whiteSpace: "nowrap" }}>
                      {dateTime(r.created_at, t.locale)}
                    </TableCell>
                    <TableCell padding="none">
                      <Tooltip
                        title={done || r.status === "failed" ? t.history.remove : t.history.cancel}
                      >
                        <IconButton
                          size="small"
                          aria-label={t.history.removeLabel(r.run_id)}
                          onClick={() => remove(r)}
                          sx={{ mr: 1 }}
                        >
                          <DeleteOutlineRoundedIcon fontSize="small" />
                        </IconButton>
                      </Tooltip>
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </TableContainer>
      )}
    </Stack>
  );
}
