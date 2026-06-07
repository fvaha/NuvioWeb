import express from 'express';
import cors from 'cors';
import { makeProviders, makeStandardFetcher, targets } from '@movie-web/providers';

const app = express();
app.use(cors());

const providers = makeProviders({
    fetcher: makeStandardFetcher(fetch),
    target: targets.ANY,
});

app.get('/extract', async (req, res) => {
    try {
        const { mediaType, tmdbId, imdbId, season, episode } = req.query;
        
        if (!tmdbId || !mediaType) {
            return res.status(400).json({ error: 'Missing tmdbId or mediaType (movie or tv)' });
        }

        const media = {
            type: mediaType === 'tv' ? 'show' : 'movie',
            title: 'Unknown',
            tmdbId: tmdbId,
        };
        
        if (imdbId) {
             media.imdbId = imdbId;
        }

        if (media.type === 'show') {
            if (!season || !episode) {
                return res.status(400).json({ error: 'Missing season or episode for TV show' });
            }
            media.season = { number: parseInt(season, 10), tmdbId: '' };
            media.episode = { number: parseInt(episode, 10), tmdbId: '' };
        }

        console.log(`[Scraper] Extracting: ${mediaType} ${tmdbId} (imdb: ${imdbId || 'N/A'}) from source: ${source || 'ALL'}`);
        
        let sourceFilters = [];
        if (source) {
            if (source === 'vidsrc') sourceFilters = ['vidsrc', 'vidsrcru', 'vidsrcsu'];
            if (source === 'vidcloud') sourceFilters = ['upcloud', 'rabbit', 'vidcloud'];
            if (source === 'superembed') sourceFilters = ['superembed', 'multiembed'];
            if (source === 'flixtor') sourceFilters = ['flixtor'];
        }

        const result = await providers.runAll({
            media,
            sourceFilters
        });

        if (result && result.stream) {
            console.log(`[Scraper] SUCCESS: Found stream from ${result.sourceId}`);
            // If it's a file stream, return it. If it's HLS, return it.
            return res.json({ 
                stream: result.stream, 
                sourceId: result.sourceId,
                url: result.stream[0].playlist || result.stream[0].file 
            });
        } else {
            console.log(`[Scraper] FAILED: No streams found for ${tmdbId}. Providers tried: ${providers.listSources().map(s => s.id).join(', ')}`);
            return res.status(404).json({ error: 'No streams found' });
        }
    } catch (error) {
        console.error('[Scraper] CRITICAL ERROR:', error);
        res.status(500).json({ error: error.message || 'Extraction failed' });
    }
});

const PORT = 3000;
app.listen(PORT, () => {
    console.log(`Scraper service running on port ${PORT}`);
});
