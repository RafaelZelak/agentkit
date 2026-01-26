package funcs

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/RafaelZelak/agentkit/sdk"
)

func init() {
	sdk.RegisterGoFunction("absurdo.mensagem", MensagemAbsurda)
	sdk.RegisterGoFunction("absurdo.piada", PiadaRuim)
	sdk.RegisterGoFunction("absurdo.animal", AnimalAleatorio)
}

// MensagemAbsurda retorna uma mensagem completamente aleatória e absurda
func MensagemAbsurda() string {
	mensagens := []string{
		"🦄 Um unicórnio dançando salsa no espaço!",
		"🍕 A pizza está falando com as estrelas!",
		"🐸 Sapos cantando ópera em Marte!",
		"🚀 Foguetes feitos de queijo suíço!",
		"🎸 Guitarra tocando música de gelatina!",
		"🌮 Taco que se transforma em borboleta!",
		"🎭 Máscaras que contam piadas em japonês!",
		"🦜 Papagaio que programa em Python!",
	}
	
	rand.Seed(time.Now().UnixNano())
	return mensagens[rand.Intn(len(mensagens))]
}

// PiadaRuim retorna uma piada terrível (com parâmetro opcional)
func PiadaRuim(tipo string) string {
	if tipo == "" {
		tipo = "geral"
	}
	
	piadas := map[string][]string{
		"geral": {
			"Por que o peixe não usa computador? Porque ele já tem um peixe-nel! 🐟",
			"O que o tomate foi fazer no banco? Foi tirar extrato! 🍅",
			"Por que a galinha atravessou a rua? Para chegar do outro lado! 🐔",
		},
		"tecnologia": {
			"Por que o programador foi ao médico? Porque tinha um bug! 🐛",
			"O que o Java disse para o C++? Você não tem classe! ☕",
			"Por que o banco de dados foi ao psicólogo? Porque tinha problemas de relacionamento! 💾",
		},
		"comida": {
			"Por que o pão foi ao médico? Porque estava com dor de cabeça! 🍞",
			"O que o queijo disse para o pão? Você é muito mole! 🧀",
			"Por que o café foi preso? Porque era um assaltante! ☕",
		},
	}
	
	lista, ok := piadas[tipo]
	if !ok {
		lista = piadas["geral"]
	}
	
	rand.Seed(time.Now().UnixNano())
	return lista[rand.Intn(len(lista))]
}

// AnimalAleatorio retorna um animal aleatório fazendo algo absurdo
func AnimalAleatorio() string {
	animais := []string{
		"🐼 panda",
		"🦁 leão",
		"🐸 sapo",
		"🦄 unicórnio",
		"🐙 polvo",
		"🦉 coruja",
		"🐧 pinguim",
		"🦊 raposa",
	}
	
	acoes := []string{
		"tocando violino",
		"fazendo parkour",
		"programando em Go",
		"comendo pizza",
		"dançando breakdance",
		"lendo um livro",
		"assistindo Netflix",
		"fazendo yoga",
	}
	
	rand.Seed(time.Now().UnixNano())
	animal := animais[rand.Intn(len(animais))]
	acao := acoes[rand.Intn(len(acoes))]
	
	return fmt.Sprintf("Um %s %s! Absurdo, não? 😂", animal, acao)
}
