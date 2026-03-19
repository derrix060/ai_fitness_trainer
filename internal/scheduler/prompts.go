package scheduler

const MorningBriefingPrompt = `Good morning! Please give me my daily training briefing:
1. Check my training plan for today on Intervals.icu — what workout is scheduled?
2. Check my recent wellness data (sleep, soreness, HRV, mood).
3. Look at my recent training load (ATL/CTL/TSB) and fatigue.
4. Based on all this, tell me what I should do today.
5. Ask me how I'm feeling and if anything needs adjusting.
Keep it concise — this is my morning check-in.`

const ActivityAnalysisPrompt = `Check Intervals.icu for any activities from the last 24 hours (use oldest=%s).

%s

For EACH new activity, start your response with ANALYZED:<activity_id> on its own line (e.g. ANALYZED:i129194330), then provide a detailed analysis:

1. **Activity summary**: type, duration, distance, average HR, average power/pace
2. **Training classification**: what kind of training was this? (recovery, endurance/zone 2, tempo/sweetspot, threshold, VO2max, anaerobic, sprint, strength, etc.). Explain WHY you classified it this way based on the intensity distribution, HR zones, and power/pace data.
3. **Performance assessment**: how well did it go? Compare to recent similar activities. Look at pacing consistency, cardiac drift, power/pace decoupling, RPE vs actual intensity.
4. **Scientific context**: use WebSearch to find relevant exercise science research (peer-reviewed papers, systematic reviews) that supports your analysis.
5. **Key takeaways**: 2-3 actionable insights for future training.

For every scientific claim, include a reference with author, year, journal, and the finding. Format references as a numbered list at the end.

If there are NO new activities to analyze, respond with exactly: NO_NEW_ACTIVITIES`
