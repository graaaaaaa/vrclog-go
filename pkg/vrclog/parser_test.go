package vrclog_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vrclog/vrclog-go/pkg/vrclog"
	"github.com/vrclog/vrclog-go/pkg/vrclog/event"
)

func TestDefaultParser_StandardLog(t *testing.T) {
	p := vrclog.DefaultParser{}
	ctx := context.Background()

	tests := []struct {
		name      string
		line      string
		wantMatch bool
		wantType  event.Type
	}{
		{
			name:      "player_join",
			line:      "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser",
			wantMatch: true,
			wantType:  event.PlayerJoin,
		},
		{
			name:      "player_left",
			line:      "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerLeft TestUser",
			wantMatch: true,
			wantType:  event.PlayerLeft,
		},
		{
			name:      "world_join",
			line:      "2024.01.15 23:59:59 Log        -  [Behaviour] Entering Room: Test World",
			wantMatch: true,
			wantType:  event.WorldJoin,
		},
		{
			name:      "unrecognized",
			line:      "random text",
			wantMatch: false,
		},
		{
			name:      "empty",
			line:      "",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.ParseLine(ctx, tt.line)
			require.NoError(t, err)
			assert.Equal(t, tt.wantMatch, result.Matched)
			if tt.wantMatch {
				require.Len(t, result.Events, 1)
				assert.Equal(t, tt.wantType, result.Events[0].Type)
			} else {
				assert.Empty(t, result.Events)
			}
		})
	}
}

func TestParserFunc(t *testing.T) {
	called := false
	p := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		called = true
		assert.Equal(t, "test line", line)
		return vrclog.ParseResult{Matched: true}, nil
	})

	result, err := p.ParseLine(context.Background(), "test line")
	require.NoError(t, err)
	assert.True(t, called)
	assert.True(t, result.Matched)
}

func TestParserChain_ChainAll(t *testing.T) {
	p1 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		return vrclog.ParseResult{
			Events:  []event.Event{{Type: "type1"}},
			Matched: true,
		}, nil
	})
	p2 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		return vrclog.ParseResult{
			Events:  []event.Event{{Type: "type2"}},
			Matched: true,
		}, nil
	})

	chain := &vrclog.ParserChain{
		Mode:    vrclog.ChainAll,
		Parsers: []vrclog.Parser{p1, p2},
	}

	result, err := chain.ParseLine(context.Background(), "test")
	require.NoError(t, err)
	assert.True(t, result.Matched)
	assert.Len(t, result.Events, 2)
	assert.Equal(t, event.Type("type1"), result.Events[0].Type)
	assert.Equal(t, event.Type("type2"), result.Events[1].Type)
}

func TestParserChain_ChainFirst(t *testing.T) {
	callOrder := []int{}
	p1 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		callOrder = append(callOrder, 1)
		return vrclog.ParseResult{
			Events:  []event.Event{{Type: "type1"}},
			Matched: true,
		}, nil
	})
	p2 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		callOrder = append(callOrder, 2)
		return vrclog.ParseResult{
			Events:  []event.Event{{Type: "type2"}},
			Matched: true,
		}, nil
	})

	chain := &vrclog.ParserChain{
		Mode:    vrclog.ChainFirst,
		Parsers: []vrclog.Parser{p1, p2},
	}

	result, err := chain.ParseLine(context.Background(), "test")
	require.NoError(t, err)
	assert.True(t, result.Matched)
	assert.Len(t, result.Events, 1)
	assert.Equal(t, []int{1}, callOrder) // p2 should not be called
}

func TestParserChain_ChainFirst_NoMatch(t *testing.T) {
	p1 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		return vrclog.ParseResult{Matched: false}, nil
	})
	p2 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		return vrclog.ParseResult{
			Events:  []event.Event{{Type: "type2"}},
			Matched: true,
		}, nil
	})

	chain := &vrclog.ParserChain{
		Mode:    vrclog.ChainFirst,
		Parsers: []vrclog.Parser{p1, p2},
	}

	result, err := chain.ParseLine(context.Background(), "test")
	require.NoError(t, err)
	assert.True(t, result.Matched)
	assert.Len(t, result.Events, 1)
	assert.Equal(t, event.Type("type2"), result.Events[0].Type)
}

func TestParserChain_ChainContinueOnError(t *testing.T) {
	p1 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		return vrclog.ParseResult{}, errors.New("p1 error")
	})
	p2 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		return vrclog.ParseResult{
			Events:  []event.Event{{Type: "type2"}},
			Matched: true,
		}, nil
	})

	chain := &vrclog.ParserChain{
		Mode:    vrclog.ChainContinueOnError,
		Parsers: []vrclog.Parser{p1, p2},
	}

	result, err := chain.ParseLine(context.Background(), "test")
	assert.Error(t, err) // Error should be returned
	assert.Contains(t, err.Error(), "p1 error")
	assert.True(t, result.Matched) // p2's result should be included
	assert.Len(t, result.Events, 1)
}

func TestParserChain_ChainContinueOnError_AllErrors(t *testing.T) {
	p1 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		return vrclog.ParseResult{}, errors.New("p1 error")
	})
	p2 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		return vrclog.ParseResult{}, errors.New("p2 error")
	})

	chain := &vrclog.ParserChain{
		Mode:    vrclog.ChainContinueOnError,
		Parsers: []vrclog.Parser{p1, p2},
	}

	result, err := chain.ParseLine(context.Background(), "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "p1 error")
	assert.Contains(t, err.Error(), "p2 error")
	assert.False(t, result.Matched)
	assert.Empty(t, result.Events)
}

func TestParserChain_NoMatch(t *testing.T) {
	p := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		return vrclog.ParseResult{Matched: false}, nil
	})

	chain := &vrclog.ParserChain{
		Mode:    vrclog.ChainAll,
		Parsers: []vrclog.Parser{p},
	}

	result, err := chain.ParseLine(context.Background(), "test")
	require.NoError(t, err)
	assert.False(t, result.Matched)
	assert.Empty(t, result.Events)
}

func TestParserChain_Empty(t *testing.T) {
	chain := &vrclog.ParserChain{
		Mode:    vrclog.ChainAll,
		Parsers: []vrclog.Parser{},
	}

	result, err := chain.ParseLine(context.Background(), "test")
	require.NoError(t, err)
	assert.False(t, result.Matched)
	assert.Empty(t, result.Events)
}

func TestParserChain_ErrorStopsChainAll(t *testing.T) {
	callOrder := []int{}
	p1 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		callOrder = append(callOrder, 1)
		return vrclog.ParseResult{}, errors.New("error")
	})
	p2 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		callOrder = append(callOrder, 2)
		return vrclog.ParseResult{Matched: true}, nil
	})

	chain := &vrclog.ParserChain{
		Mode:    vrclog.ChainAll,
		Parsers: []vrclog.Parser{p1, p2},
	}

	_, err := chain.ParseLine(context.Background(), "test")
	assert.Error(t, err)
	assert.Equal(t, []int{1}, callOrder) // p2 should not be called
}
