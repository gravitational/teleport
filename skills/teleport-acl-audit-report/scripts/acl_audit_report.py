#!/usr/bin/env python3
"""Generate a PDF report from `tctl acl audit summary --format=json` output."""

import json
import subprocess
import sys
from collections import defaultdict
from datetime import datetime, timezone

from reportlab.lib import colors
from reportlab.lib.pagesizes import letter
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.lib.units import inch
from reportlab.platypus import (
    SimpleDocTemplate,
    Paragraph,
    Spacer,
    HRFlowable,
    KeepTogether,
    Table,
    TableStyle,
)


def fetch_audit_data():
    result = subprocess.run(
        ["tctl", "acl", "audit", "summary", "--format=json"],
        capture_output=True,
        text=True,
        check=True,
    )
    return json.loads(result.stdout)


def parse_timestamp(ts: str) -> datetime:
    return datetime.fromisoformat(ts.replace("Z", "+00:00"))


def format_dt(dt: datetime) -> str:
    return dt.strftime("%Y-%m-%d %H:%M UTC")


def build_summary(reviews: list[dict]) -> dict:
    if not reviews:
        return {}

    timestamps = [parse_timestamp(r["review_timestamp"]) for r in reviews]
    all_reviewers = set()
    total_removed = 0
    unique_lists = set()

    for r in reviews:
        unique_lists.add(r["access_list_name"])
        all_reviewers.update(r.get("reviewers") or [])
        total_removed += len(r.get("removed_members") or [])

    return {
        "period_start": min(timestamps),
        "period_end": max(timestamps),
        "total_lists": len(unique_lists),
        "total_removed": total_removed,
        "total_reviewers": len(all_reviewers),
    }


def group_by_list(reviews: list[dict]) -> dict:
    """Group reviews by access list, preserving chronological order per list."""
    groups = defaultdict(list)
    for r in reviews:
        groups[r["access_list_name"]].append(r)
    # Sort each group newest-first
    for name in groups:
        groups[name].sort(key=lambda r: parse_timestamp(r["review_timestamp"]), reverse=True)
    # Return ordered by each list's most recent review, newest first
    seen = []
    for r in reviews:
        name = r["access_list_name"]
        if name not in seen:
            seen.append(name)
    seen.sort(key=lambda name: parse_timestamp(groups[name][0]["review_timestamp"]), reverse=True)
    return {name: groups[name] for name in seen}


def make_styles():
    base = getSampleStyleSheet()

    title = ParagraphStyle(
        "ReportTitle",
        parent=base["Title"],
        fontSize=22,
        spaceAfter=6,
        textColor=colors.HexColor("#1a1a2e"),
    )
    subtitle = ParagraphStyle(
        "ReportSubtitle",
        parent=base["Normal"],
        fontSize=10,
        textColor=colors.HexColor("#555555"),
        spaceAfter=4,
    )
    section_header = ParagraphStyle(
        "SectionHeader",
        parent=base["Heading1"],
        fontSize=14,
        textColor=colors.HexColor("#1a1a2e"),
        spaceBefore=16,
        spaceAfter=8,
        borderPad=0,
    )
    list_title = ParagraphStyle(
        "ListTitle",
        parent=base["Heading2"],
        fontSize=12,
        textColor=colors.HexColor("#0f3460"),
        spaceBefore=14,
        spaceAfter=2,
    )
    list_description = ParagraphStyle(
        "ListDescription",
        parent=base["Normal"],
        fontSize=9,
        textColor=colors.HexColor("#666666"),
        spaceAfter=6,
        leftIndent=0,
        italic=True,
    )
    review_header = ParagraphStyle(
        "ReviewHeader",
        parent=base["Normal"],
        fontSize=10,
        textColor=colors.HexColor("#333333"),
        spaceBefore=8,
        spaceAfter=2,
        fontName="Helvetica-Bold",
    )
    field_label = ParagraphStyle(
        "FieldLabel",
        parent=base["Normal"],
        fontSize=9,
        textColor=colors.HexColor("#888888"),
        spaceAfter=1,
        leftIndent=12,
        fontName="Helvetica-Bold",
    )
    field_value = ParagraphStyle(
        "FieldValue",
        parent=base["Normal"],
        fontSize=9,
        textColor=colors.HexColor("#222222"),
        spaceAfter=4,
        leftIndent=12,
    )
    notes_style = ParagraphStyle(
        "Notes",
        parent=base["Normal"],
        fontSize=9,
        textColor=colors.HexColor("#444444"),
        spaceAfter=4,
        leftIndent=12,
        backColor=colors.HexColor("#f5f5f0"),
        borderPad=4,
    )
    table_header = ParagraphStyle(
        "TableHeader",
        parent=base["Normal"],
        fontSize=9,
        textColor=colors.white,
        fontName="Helvetica-Bold",
    )
    table_cell = ParagraphStyle(
        "TableCell",
        parent=base["Normal"],
        fontSize=9,
        textColor=colors.HexColor("#222222"),
    )
    return {
        "title": title,
        "subtitle": subtitle,
        "section_header": section_header,
        "list_title": list_title,
        "list_description": list_description,
        "review_header": review_header,
        "field_label": field_label,
        "field_value": field_value,
        "notes": notes_style,
        "table_header": table_header,
        "table_cell": table_cell,
    }


def build_pdf(reviews: list[dict], output_path: str):
    doc = SimpleDocTemplate(
        output_path,
        pagesize=letter,
        leftMargin=inch,
        rightMargin=inch,
        topMargin=inch,
        bottomMargin=inch,
    )

    styles = make_styles()
    summary = build_summary(reviews)
    groups = group_by_list(reviews)
    story = []

    # ── Title ──────────────────────────────────────────────────────────────────
    story.append(Paragraph("Access List Audit Report", styles["title"]))
    story.append(Spacer(1, 6))
    generated_at = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M UTC")
    story.append(Paragraph(f"Generated {generated_at}", styles["subtitle"]))
    story.append(HRFlowable(width="100%", thickness=1, color=colors.HexColor("#cccccc")))
    story.append(Spacer(1, 12))

    # ── Summary section ────────────────────────────────────────────────────────
    story.append(Paragraph("Reviews Summary", styles["section_header"]))

    def fmt_date(dt: datetime) -> str:
        return dt.strftime("%Y-%m-%d")

    period = (
        f"{fmt_date(summary['period_start'])}  –  {fmt_date(summary['period_end'])}"
        if summary
        else "N/A"
    )

    H = styles["table_header"]
    C = styles["table_cell"]

    summary_table = Table(
        [
            [Paragraph("Reporting Period", H), Paragraph(period, C)],
            [Paragraph("Access Lists Reviewed", H), Paragraph(str(summary.get("total_lists", 0)), C)],
            [Paragraph("Members Removed", H), Paragraph(str(summary.get("total_removed", 0)), C)],
            [Paragraph("Unique Reviewers", H), Paragraph(str(summary.get("total_reviewers", 0)), C)],
        ],
        colWidths=[2.2 * inch, 4.3 * inch],
        hAlign="LEFT",
    )
    summary_table.setStyle(TableStyle([
        ("BACKGROUND", (0, 0), (0, -1), colors.HexColor("#0f3460")),
        ("ROWBACKGROUNDS", (1, 0), (1, -1), [colors.HexColor("#f0f4f8"), colors.white]),
        ("BOX", (0, 0), (-1, -1), 0.5, colors.HexColor("#cccccc")),
        ("INNERGRID", (0, 0), (-1, -1), 0.5, colors.HexColor("#cccccc")),
        ("TOPPADDING", (0, 0), (-1, -1), 7),
        ("BOTTOMPADDING", (0, 0), (-1, -1), 7),
        ("LEFTPADDING", (0, 0), (-1, -1), 8),
        ("RIGHTPADDING", (0, 0), (-1, -1), 8),
        ("VALIGN", (0, 0), (-1, -1), "MIDDLE"),
        ("FONTSIZE", (1, 0), (1, -1), 11),
        ("FONTNAME", (1, 0), (1, -1), "Helvetica-Bold"),
        ("TEXTCOLOR", (1, 0), (1, -1), colors.HexColor("#0f3460")),
    ]))
    story.append(summary_table)

    story.append(Spacer(1, 16))
    story.append(HRFlowable(width="100%", thickness=1, color=colors.HexColor("#cccccc")))

    # ── Per-list review activity ───────────────────────────────────────────────
    story.append(Paragraph("Review Activity", styles["section_header"]))

    for list_name, list_reviews in groups.items():
        first = list_reviews[0]
        title_text = first.get("title") or list_name
        description = first.get("description") or ""

        block = []
        block.append(Paragraph(title_text, styles["list_title"]))
        if description:
            block.append(Paragraph(description, styles["list_description"]))

        for i, review in enumerate(list_reviews):
            dt = format_dt(parse_timestamp(review["review_timestamp"]))
            reviewers = ", ".join(review.get("reviewers") or []) or "—"
            removed = ", ".join(review.get("removed_members") or []) or "None"
            notes = (review.get("notes") or "").strip()

            block.append(Paragraph(dt, styles["review_header"]))

            block.append(Paragraph("Reviewers", styles["field_label"]))
            block.append(Paragraph(reviewers, styles["field_value"]))

            block.append(Paragraph("Members Removed", styles["field_label"]))
            block.append(Paragraph(removed, styles["field_value"]))

            if notes:
                block.append(Paragraph("Notes", styles["field_label"]))
                block.append(Paragraph(notes, styles["notes"]))

        block.append(
            HRFlowable(width="100%", thickness=0.5, color=colors.HexColor("#dddddd"), spaceAfter=4)
        )
        story.append(KeepTogether(block))

    doc.build(story)
    print(f"Report written to: {output_path}")


def main():
    output = sys.argv[1] if len(sys.argv) > 1 else "acl_audit_report.pdf"
    print("Fetching audit summary…")
    reviews = fetch_audit_data()
    if not reviews:
        print("No audit data returned.")
        sys.exit(0)
    print(f"Processing {len(reviews)} review record(s)…")
    build_pdf(reviews, output)


if __name__ == "__main__":
    main()
